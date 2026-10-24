/* eslint-disable @next/next/no-img-element */
import Link from "next/link";
import QRCode from "qrcode";
import "./landing.css";

// The logo is rendered exactly as supplied (no re-encoding; 01 §3.1).
// Copy rules: 04 §7.3 (British English, short declaratives, no hype words) and
// claims discipline 01 §15.1 (never "safe", "vetted" or "background-checked").


function PhoneApprove() {
  return (
    <div className="device-phone" aria-hidden="true">
      <div className="device-screen">
        <p className="eyebrow">Approve sign-in</p>
        <h4>Is this you?</h4>
        <p>Someone is signing in to your account on a computer.</p>
        <div className="meta">
          <div><span>Browser</span><b>Chrome on Windows</b></div>
          <div><span>Near</span><b>London</b></div>
          <div><span>When</span><b>Just now</b></div>
        </div>
        <span className="button">Approve with Face ID</span>
        <span className="button ghost">It wasn&rsquo;t me</span>
      </div>
    </div>
  );
}

function PhoneDecoy() {
  return (
    <div className="device-phone" aria-hidden="true">
      <div className="device-screen">
        <p className="eyebrow">Discretion</p>
        <h4>Your rules</h4>
        <div className="meta">
          <div><span>Notifications</span><b>Masked</b></div>
          <div><span>Messages expire</span><b>30 days</b></div>
          <div><span>Hidden near</span><b>Home, work</b></div>
          <div><span>Partner shield</span><b>2 contacts</b></div>
          <div><span>Discovery</span><b>Paused</b></div>
        </div>
        <span className="button">Burn this device</span>
        <p style={{ marginTop: 14, fontSize: 11 }}>Panic is a separate control. It calls for help and keeps what responders need.</p>
      </div>
    </div>
  );
}

function LaptopSignIn({ qrSvg }: { qrSvg: string }) {
  return (
    <div className="device-laptop" aria-hidden="true">
      <div className="lid">
        <div className="glass">
          <div className="row">
            <div>
              <p className="eyebrow">Sign in</p>
              <h4>Approve on your phone</h4>
              <ul className="steps-mini">
                <li><span>I. Your passkey</span><b>Done</b></li>
                <li><span>II. Your phone</span><b>Waiting</b></li>
                <li><span>Code expires in</span><b>0:48</b></li>
              </ul>
            </div>
            <div className="qr-mini" dangerouslySetInnerHTML={{ __html: qrSvg }} />
          </div>
        </div>
      </div>
      <div className="base" />
    </div>
  );
}

export default async function Home() {
  // A real, scannable code in the mock-up (it encodes a sample approval address).
  const qrSvg = await QRCode.toString("TRYST sign-in approval (example)", {
    type: "svg", margin: 0, color: { dark: "#0B0607", light: "#FCF2DD" },
  });
  return (
    <div className="lp">
      <header className="lp-topbar">
        <Link className="mark" href="/" aria-label="TRYST home">TRYST</Link>
        <nav aria-label="Primary">
          <a href="#how">How it works</a>
          <a href="#membership">Membership</a>
          <Link href="/sign-in">Sign in</Link>
        </nav>
      </header>

      {/* I. Hero */}
      <section className="lp-section lp-hero">
        <div>
          <img className="logo reveal" src="/brand/tryst-logo-512.webp" alt="TRYST" width={512} height={512} fetchPriority="high" />
          <p className="eyebrow reveal d1">For attached adults, open relationships and couples</p>
          <h1 className="reveal d2">Private chemistry.<br /><em>Intelligent discretion.</em></h1>
          <p className="lede reveal d3">
            A members&rsquo; club for one-off encounters, ongoing affairs, open relationships, thirds and couples. Every member is a
            verified adult. When interest is mutual, you each see what you share before anything opens. And the whole
            thing is built to forget.
          </p>
          <div className="lp-ctas reveal d4">
            <Link className="button" href="/join">Become a member</Link>
            <Link className="button ghost" href="/sign-in">Sign in</Link>
          </div>
          <p className="lp-scrollcue reveal d4">London, Manchester, Birmingham and Brighton</p>
        </div>
      </section>

      {/* II. Manifesto */}
      <section className="lp-section lp-manifesto">
        <blockquote>Discretion is the product.</blockquote>
        <p>
          We don&rsquo;t ask whether your partner knows. That&rsquo;s your business. We are absolute about consent
          and conduct between the people who are here.
        </p>
      </section>

      {/* III. Principles */}
      <section className="lp-section">
        <div className="lp-wrap">
          <p className="eyebrow">What we hold to</p>
          <div className="lp-grid3">
            <article className="lp-principle">
              <span className="num">I</span>
              <h3>Real, verified adults</h3>
              <p>
                Every member passes a live face check and age assurance before they can see anyone. A couple
                profile exists only when both partners confirm it, each from their own phone.
              </p>
            </article>
            <article className="lp-principle">
              <span className="num">II</span>
              <h3>A brief before anything opens</h3>
              <p>
                When interest is mutual, each of you gets a short brief: what you both want and what you share.
                Nobody sees the other&rsquo;s decision, and nothing opens until you both say yes.
              </p>
            </article>
            <article className="lp-principle">
              <span className="num">III</span>
              <h3>Built to forget</h3>
              <p>
                Messages are end-to-end encrypted, so we cannot read them. Delete your account and it&rsquo;s removed
                straight away, with a signed proof. There are no adverts and no trackers.
              </p>
            </article>
          </div>
        </div>
      </section>

      {/* IV. The evening */}
      <section className="lp-section" id="how">
        <div className="lp-wrap">
          <p className="eyebrow">How an evening comes together</p>
          <h2>Fewer messages. More evenings.</h2>
          <ol className="lp-sequence">
            <li><span className="t">Each morning</span><h3>A short list</h3><p>Up to eighteen introductions, chosen for fit. No endless scrolling.</p></li>
            <li><span className="t">When it&rsquo;s mutual</span><h3>The handshake</h3><p>Reach out with a private intent. If they reply, you&rsquo;re both shown a brief.</p></li>
            <li><span className="t">Your brief</span><h3>You decide</h3><p>See what you both want and what you share. Accept or decline.</p></li>
            <li><span className="t">Both accept</span><h3>A private thread</h3><p>Encrypted from end to end. Messages expire on your schedule.</p></li>
            <li><span className="t">On the night</span><h3>Meet well</h3><p>Public places first, a quiet check-in timer, and a plan only your chosen contact sees.</p></li>
          </ol>
        </div>
      </section>

      {/* V. Modes */}
      <section className="lp-section">
        <div className="lp-wrap">
          <p className="eyebrow">Say what you&rsquo;re here for</p>
          <h2>Five ways in.</h2>
          <div className="lp-modes">
            <div className="lp-mode"><div className="name">SPARK</div><div className="who">One evening</div><p>A single, discreet encounter.</p></div>
            <div className="lp-mode"><div className="name">EMBER</div><div className="who">Something ongoing</div><p>A recurring connection with agreed rhythm.</p></div>
            <div className="lp-mode"><div className="name">THIRD</div><div className="who">Couples and a third</div><p>Both partners verified. The third has an equal say.</p></div>
            <div className="lp-mode"><div className="name">QUAD</div><div className="who">Couple to couple</div><p>Four people, four verified accounts. Each couple decides how it says yes.</p></div>
            <div className="lp-mode"><div className="name">OPEN</div><div className="who">Open relationships</div><p>Openly non-monogamous, or still exploring. Set your own lines.</p></div>
          </div>
        </div>
      </section>

      {/* VI. Discretion */}
      <section className="lp-section">
        <div className="lp-wrap lp-split">
          <div>
            <p className="eyebrow">Discretion, engineered</p>
            <h2>Designed for the phone in someone else&rsquo;s hand.</h2>
            <ul className="lp-list">
              <li><strong>Leave in an instant</strong><span>One tap or a double Escape clears the screen and takes you somewhere ordinary.</span></li>
              <li><strong>Burn</strong><span>Wipes this device, revokes its keys and signs you out everywhere.</span></li>
              <li><strong>Never near home</strong><span>Your profile hides itself inside the home and work areas you set, and from up to ten people whose email you add.</span></li>
            </ul>
            <p className="lp-honest">
              What we can&rsquo;t hide: your device, your bank, your network, or a screenshot someone else takes.
              We tell you that plainly because the rest depends on trusting us.
            </p>
          </div>
          <PhoneDecoy />
        </div>
      </section>

      {/* VII. Sign-in */}
      <section className="lp-section">
        <div className="lp-wrap lp-signin">
          <div>
            <p className="eyebrow">Only you can sign in</p>
            <h2>Two checks, every time.</h2>
            <p className="lede">
              On your phone: your fingerprint or face, then a live face check. On a computer: your passkey, then
              approval from your phone. No passwords, no text-message codes, and no one at TRYST can sign in for you.
            </p>
          </div>
          <div className="lp-devices">
            <LaptopSignIn qrSvg={qrSvg} />
            <PhoneApprove />
          </div>
        </div>
      </section>

      {/* VIII. Membership */}
      <section className="lp-section" id="membership">
        <div className="lp-wrap">
          <p className="eyebrow">Membership</p>
          <h2>Free to join. One paid plan.</h2>
          <p className="lede">Replying is always free. TRYST+ removes the weekly limit on reaching out.</p>
          <div className="lp-tiers">
            <div className="lp-tier">
              <div className="tname">Verified</div>
              <div className="price">£0</div><div className="per">free</div>
              <ul><li>Full profile</li><li>Daily introductions</li><li>Reply to anyone, free</li><li>Three private intents a week</li><li>Every safety feature</li></ul>
            </div>
            <div className="lp-tier">
              <div className="tname">TRYST+</div>
              <div className="price">£24.99</div><div className="per">a month · or £179.88 a year (£14.99 a month)</div>
              <ul><li>Everything in Verified</li><li>Unlimited private intents</li></ul>
            </div>
          </div>
          <div className="lp-extras">
            <div><b>Cancel in one step</b>From your account. No calls, no questions. Cancel within 14 days for a refund of the unused part. Prices shown are for the UK.</div>
          </div>
        </div>
      </section>

      {/* IX. Before you meet */}
      <section className="lp-section lp-safety">
        <div className="lp-wrap lp-narrow">
          <p className="eyebrow">Read this before you meet anyone</p>
          <p>
            We verify that members are real adults. We do not, and cannot, check anyone&rsquo;s criminal record, their
            honesty, or whether they are who they say they are once you are off this app. No UK platform can. Meet in
            public the first time. Tell someone where you are going; Share My Plan does it privately. Trust your
            instincts and leave if something feels wrong. If anyone pressures you, threatens you, or pushes past a
            boundary you set, report them. We act on it, and we do it fast.
          </p>
        </div>
      </section>

      {/* X. Close */}
      <section className="lp-section lp-final">
        <div className="lp-wrap lp-narrow">
          <h2>Your evening,<br /><em>on your terms.</em></h2>
          <p className="lede" style={{ margin: "0 auto 2.4rem" }}>Open to verified adults in London, Manchester, Birmingham and Brighton.</p>
          <div className="lp-ctas">
            <Link className="button" href="/join">Become a member</Link>
          </div>
        </div>
      </section>

      <footer className="footer">
        <span>TRYST is for adults aged 18 and over. Not available in every country.</span>
        <span>Terms, privacy notice and safety policy are published at launch.</span>
      </footer>
    </div>
  );
}
