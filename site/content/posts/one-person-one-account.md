---
title: One person, one account
description: The same ID can't open a second TRYST account, and people we permanently remove can't simply sign up again under a new email.
date: 2026-10-22
topic: verification
usp: A removed member can't come back with a new email address, because accounts are tied to an identity document, not an inbox.
status: live
order: 3
---
On most platforms, a banned account is an inconvenience: make a new email address and start again. On TRYST, it doesn't work like that.

## The problem

Fake and duplicate accounts are how scammers scale and how people who have been removed for harmful behaviour find their way back. If anyone can open as many accounts as they have email addresses, any ban is temporary.

## How it works

When you verify, our provider checks your ID document and sends TRYST its number. We immediately turn that number into a **one-way code**, keyed with a secret held by TRYST alone, and throw the number away. The code can't be turned back into your document number, but the same document always produces the same code.

That gives us three things without storing anyone's ID:

- **One account per document.** If a document already holds an account, a second one is refused. (If it's yours, you [recover it](/blog/account-recovery-without-a-back-door/) instead.)
- **Bans that last.** If someone is permanently removed, their document's code goes on a block list that survives even when their account is deleted. That's the one exception to our deletion promise, and our Terms say so.
- **Recovery that proves it's you.** Getting back into a lost account means checking the same document again.

## Why it's different

Most "one account each" rules depend on email addresses, phone numbers or device fingerprints, all of which are easy to change or fake. Tying accounts to a **verified document**, while keeping only a one-way code, gives the protection of an ID register without building one.

## What it doesn't do

- It can't stop someone using a different genuine document they legitimately hold, such as a passport and a driving licence. Re-verification has to use the same document as before.
- The code identifies a document, not a person's behaviour. Removal decisions are still made by people, with appeals ([fair moderation](/blog/report-block-and-fair-moderation/)).

## In short

One verified person, one account. Removed means removed.
