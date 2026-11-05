# Submission material — customise before posting

## Project title

**Vyapari Voice Copilot — Speak. Sell. Restock.**

## One-line description

A Telugu + English voice operations copilot that lets small Indian shopkeepers record sales, check inventory and receive demand-aware reorder suggestions without typing.

## Problem

Many small retailers still manage inventory through memory, notebooks or software that requires frequent typing and menu navigation. This becomes harder while serving customers, especially for users who naturally speak in a regional language mixed with English product names.

## Solution

Vyapari converts natural Telugu + English commands into validated shop actions. Gnani Prisma transcribes speech, a Go transaction engine identifies the intended operation and updates inventory, an analytics engine calculates business metrics and reorder recommendations, and Gnani Timbre speaks the result back.

Unlike a generic voice chatbot, the application changes real, persistent business state and shows every action in a traceable dashboard.

## Target users

- Kirana and neighbourhood-store owners
- Small wholesalers
- Family-run shops
- Staff who are comfortable speaking but not using complex point-of-sale software

## Gnani AI technology used

- **Gnani Prisma:** speech-to-text for Indian accents, Telugu + English code mixing and product vocabulary
- **Gnani Timbre:** Telugu and English spoken responses with Indian names and number cadence

## Key capabilities

1. Voice stock check
2. Voice sale recording
3. Voice restocking
4. Daily revenue and gross-profit summary
5. Low-stock alerts
6. Seven-day demand analysis
7. Explainable reorder suggestions
8. Spoken Telugu/English replies
9. Undo, activity history and CSV exports
10. Secure server-side API-key handling

## Technical contribution

Replace this section with your genuine contribution. Example for a solo submission:

> I designed and implemented the complete application: browser audio capture and WAV encoding, Go REST backend, Gnani Prisma and Timbre integrations, multilingual intent parsing, inventory transactions, persistence, analytics, responsive interface, tests, documentation and demo preparation.

Do not claim work that someone else performed.

## What makes it distinct

- It is an operational copilot, not a conversational mock-up.
- Speech changes inventory and sales state with validation and undo.
- Product names are supplied as STT context/keyterms.
- The app works with Telugu script, English and common transliterated Telugu.
- Reorder suggestions explain demand, days of stock cover, quantity and cost.
- The API key never enters browser code.
- The public demo can show the complete voice → action → dashboard → spoken-reply loop.

## Suggested LinkedIn post

I built **Vyapari Voice Copilot**, a Telugu + English voice operations assistant for small Indian retailers.

A shopkeeper can say:

“Basmati rice rendu packets sale ayindi.”

Vyapari transcribes the code-mixed command with **Gnani Prisma**, validates and records the sale in a Go backend, updates inventory and analytics, and speaks the result back through **Gnani Timbre**.

The application also checks stock, gives daily revenue/profit summaries, detects low-stock products and creates explainable reorder recommendations from recent demand.

This is not only a voice chatbot—the speech performs safe, traceable business actions.

Built for the Great Indian AI Internship Challenge using synthetic demo data.

#GnaniAI #GreatIndianAIInternshipChallenge #VoiceAI #Golang #BuildInPublic #IndianAI

## Suggested repository description

Telugu + English voice inventory and sales copilot for small Indian retailers, built with Go, Gnani Prisma STT and Gnani Timbre TTS.

## Final checklist

- [ ] Real API key configured and hidden
- [ ] Both STT and TTS visibly work
- [ ] Demo uses synthetic data
- [ ] Public post is live before final submission
- [ ] Gnani AI is tagged
- [ ] Required hashtags are present
- [ ] Demo link opens without requesting access
- [ ] Project explanation names the exact models/APIs
- [ ] Contribution statement is truthful
- [ ] Video shows the working application, not only slides
- [ ] Submission completed before the official deadline
