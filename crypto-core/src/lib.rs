//! TRYST client crypto core (spec: docs/spec/03_Backend.md §6.2, docs/spec/04_Frontend.md §6).
//!
//! Media flow (FR-018):
//! 1. The client generates a fresh AES-256-GCM key per media item and encrypts it.
//! 2. Ciphertext goes to the object store; the server never sees plaintext.
//! 3. The media key is wrapped separately for each recipient (here under a recipient
//!    key-encryption key; in production, derived from the MLS exporter secret).
//! 4. Revoking = destroying the recipient's wrapped key. Without it the ciphertext is noise.
//!
//! Compiled to Swift (UniFFI), Kotlin (JNI) and WASM so all three surfaces share one audited
//! implementation.

use aes_gcm::aead::{Aead, KeyInit, Payload};
use aes_gcm::{Aes256Gcm, Key, Nonce};
use rand_core::{OsRng, RngCore};
use std::collections::HashMap;
use zeroize::{Zeroize, ZeroizeOnDrop};

const NONCE_LEN: usize = 12;

#[derive(Debug, PartialEq, Eq)]
pub enum CryptoError {
    Encrypt,
    Decrypt,
    Revoked,
    UnknownRecipient,
    Malformed,
}

/// A 256-bit symmetric key that is wiped from memory when dropped.
#[derive(Clone, Zeroize, ZeroizeOnDrop)]
pub struct SecretKey([u8; 32]);

impl SecretKey {
    pub fn generate() -> Self {
        let mut k = [0u8; 32];
        OsRng.fill_bytes(&mut k);
        SecretKey(k)
    }
    fn cipher(&self) -> Aes256Gcm {
        Aes256Gcm::new(Key::<Aes256Gcm>::from_slice(&self.0))
    }
}

fn seal(key: &SecretKey, plaintext: &[u8], aad: &[u8]) -> Result<Vec<u8>, CryptoError> {
    let mut nonce = [0u8; NONCE_LEN];
    OsRng.fill_bytes(&mut nonce);
    let ct = key
        .cipher()
        .encrypt(
            Nonce::from_slice(&nonce),
            Payload {
                msg: plaintext,
                aad,
            },
        )
        .map_err(|_| CryptoError::Encrypt)?;
    let mut out = nonce.to_vec();
    out.extend_from_slice(&ct);
    Ok(out)
}

fn open(key: &SecretKey, sealed: &[u8], aad: &[u8]) -> Result<Vec<u8>, CryptoError> {
    if sealed.len() < NONCE_LEN + 16 {
        return Err(CryptoError::Malformed);
    }
    let (nonce, ct) = sealed.split_at(NONCE_LEN);
    key.cipher()
        .decrypt(Nonce::from_slice(nonce), Payload { msg: ct, aad })
        .map_err(|_| CryptoError::Decrypt)
}

/// Encrypted media plus its per-recipient wrapped keys.
pub struct SealedMedia {
    pub media_id: String,
    pub ciphertext: Vec<u8>,
    wrapped: HashMap<String, Vec<u8>>,
}

impl SealedMedia {
    /// Encrypt `plaintext` under a fresh media key and wrap that key for each recipient.
    /// The media key itself is dropped (and zeroised) when this returns.
    pub fn seal(
        media_id: &str,
        plaintext: &[u8],
        recipients: &[(&str, &SecretKey)],
    ) -> Result<Self, CryptoError> {
        let media_key = SecretKey::generate();
        let ciphertext = seal(&media_key, plaintext, media_id.as_bytes())?;
        let mut wrapped = HashMap::new();
        for (rid, kek) in recipients {
            let aad = format!("{media_id}|{rid}");
            wrapped.insert((*rid).to_string(), seal(kek, &media_key.0, aad.as_bytes())?);
        }
        Ok(SealedMedia {
            media_id: media_id.to_string(),
            ciphertext,
            wrapped,
        })
    }

    /// Grant an additional recipient, given a key that can unwrap an existing grant.
    pub fn grant(
        &mut self,
        via: &str,
        via_kek: &SecretKey,
        recipient: &str,
        recipient_kek: &SecretKey,
    ) -> Result<(), CryptoError> {
        let media_key = self.unwrap(via, via_kek)?;
        let aad = format!("{}|{recipient}", self.media_id);
        self.wrapped.insert(
            recipient.to_string(),
            seal(recipient_kek, &media_key.0, aad.as_bytes())?,
        );
        Ok(())
    }

    /// Revoke = destroy the wrapped key. Irreversible for that recipient.
    pub fn revoke(&mut self, recipient: &str) {
        if let Some(mut w) = self.wrapped.remove(recipient) {
            w.zeroize();
        }
    }

    pub fn has_grant(&self, recipient: &str) -> bool {
        self.wrapped.contains_key(recipient)
    }

    fn unwrap(&self, recipient: &str, kek: &SecretKey) -> Result<SecretKey, CryptoError> {
        let w = self.wrapped.get(recipient).ok_or(CryptoError::Revoked)?;
        let aad = format!("{}|{recipient}", self.media_id);
        let raw = open(kek, w, aad.as_bytes())?;
        let bytes: [u8; 32] = raw
            .as_slice()
            .try_into()
            .map_err(|_| CryptoError::Malformed)?;
        Ok(SecretKey(bytes))
    }

    /// Decrypt as `recipient`. Fails once the grant is revoked.
    pub fn open_as(&self, recipient: &str, kek: &SecretKey) -> Result<Vec<u8>, CryptoError> {
        let media_key = self.unwrap(recipient, kek)?;
        open(&media_key, &self.ciphertext, self.media_id.as_bytes())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn recipient_can_open_until_revoked() {
        let alice = SecretKey::generate();
        let bob = SecretKey::generate();
        let mut m =
            SealedMedia::seal("m1", b"photo-bytes", &[("alice", &alice), ("bob", &bob)]).unwrap();
        assert_eq!(m.open_as("bob", &bob).unwrap(), b"photo-bytes");
        m.revoke("bob");
        assert!(!m.has_grant("bob"));
        assert_eq!(m.open_as("bob", &bob), Err(CryptoError::Revoked));
        assert_eq!(m.open_as("alice", &alice).unwrap(), b"photo-bytes");
    }

    #[test]
    fn wrong_key_cannot_open() {
        let alice = SecretKey::generate();
        let eve = SecretKey::generate();
        let m = SealedMedia::seal("m1", b"x", &[("alice", &alice)]).unwrap();
        assert_eq!(m.open_as("alice", &eve), Err(CryptoError::Decrypt));
        assert_eq!(m.open_as("eve", &eve), Err(CryptoError::Revoked));
    }

    #[test]
    fn grants_are_bound_to_media_and_recipient() {
        let alice = SecretKey::generate();
        let bob = SecretKey::generate();
        let mut m = SealedMedia::seal("m1", b"x", &[("alice", &alice)]).unwrap();
        m.grant("alice", &alice, "bob", &bob).unwrap();
        // Moving bob's wrapped key to a different recipient name must fail (AAD binding).
        let w = m.wrapped.remove("bob").unwrap();
        m.wrapped.insert("carol".into(), w);
        assert_eq!(m.open_as("carol", &bob), Err(CryptoError::Decrypt));
    }

    #[test]
    fn ciphertext_is_not_plaintext() {
        let k = SecretKey::generate();
        let m = SealedMedia::seal("m1", b"very private", &[("a", &k)]).unwrap();
        assert!(!m.ciphertext.windows(12).any(|w| w == b"very private"));
    }
}
