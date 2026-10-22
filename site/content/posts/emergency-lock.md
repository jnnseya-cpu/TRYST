---
title: "The emergency lock: lock your account without your phone"
description: If your phone is taken, lock TRYST from any browser with your email and a one-time code you got when you joined. No sign-in needed.
date: 2026-10-22
topic: security
usp: Your account can be locked from any browser in seconds, even when the phone that signs you in is in someone else's hands.
status: live
order: 19
---
What if the person you need to keep out of your account is holding your phone? Most services have no answer. TRYST has the emergency lock.

## The problem

Locking an account normally means signing in to it, which needs the very device that's been taken. By the time you get it back, or get support on the phone, the damage is done.

## How it works

- When you join, TRYST shows you a **one-time emergency code**, once. Keep it somewhere only you can reach.
- If your phone is taken, go to the lock page **from any browser**, enter your email and the code. **No sign-in needed.**
- The account is locked at once:
  - every session is signed out;
  - nobody can sign in, not even with your passkeys;
  - your profile and photos are hidden from everyone.
- To unlock, you [recover the account](/blog/account-recovery-without-a-back-door/), proving it's you with your ID. Recovery issues a **new** emergency code, because the old one may be known.

The lock page gives **the same answer whatever is entered**, and allows only five attempts a day per email, so it can't be used to find out whether someone has an account.

## Why it's different

It's a lock that works **from outside the account**, without the device you've lost control of. And it can **only lock, never expose**: someone who guesses or steals your code can lock you out, but can't see or change anything.

## What it doesn't do

- It can't lock your phone itself. Use your phone's own "find my device" service for that.
- Without the code, the lock page can't help you. That's deliberate: otherwise anyone could lock anyone.
- Unlocking takes time, because recovery includes a 24-hour wait.

## In short

One code, any browser, and your account goes dark.
