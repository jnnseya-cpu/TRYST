---
title: "Download everything, or delete for good"
description: Download all your data whenever you like. Delete your account and the key protecting your data is destroyed first, with a signed proof of what was erased.
date: 2026-10-22
topic: data
usp: Deleting your account destroys the key your data is locked with, and you get a signed proof of what was erased.
status: live
order: 23
---
Leaving a service should be as clean as joining it. On TRYST, you can take everything with you, or make sure nothing is left behind.

## The problem

"Delete account" on many platforms means a flag in a database, with your data still sitting in backups for months or years. Data breaches have exposed people who thought they'd deleted their accounts long before.

## How it works

**Download**

- One button gives you everything we hold about you: profile, photos, introductions, intents, consents, settings and conversation details.
- Because messages are [end-to-end encrypted](/blog/end-to-end-encrypted-messages/), their content is on your devices, not ours; the download includes what we hold about each conversation.
- It needs [both sign-in checks](/blog/step-up-and-your-devices/), so nobody with your unlocked phone can take a copy.

**Delete**

- Type "delete my account" to confirm. No step-up, no waiting, no persuading.
- Your data is protected with a key unique to you. **That key is destroyed first**, so everything it protected becomes unreadable at once, including in backups. Copies of the key in key backups expire within seven days.
- Then every part of TRYST removes your records, and you receive a **signed completion proof**: a document, verifiable with our published key, listing what each part erased and any exception kept.
- A live subscription ends; inside the first 14 days you're refunded for the days you didn't use.

## Why it's different

**Cryptographic erasure** means deletion doesn't depend on finding every copy: without the key, the copies are noise. And the **signed proof** turns "trust us" into something you can check.

## What it doesn't do

- If you were permanently removed, a one-way code of your ID stays on a block list so you can't rejoin. The proof says so plainly.
- It can't delete messages already on the other person's device.
- Deletion can't be undone.

## In short

Take it all, or leave nothing readable, with proof either way.
