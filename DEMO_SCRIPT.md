# 60-second public demo script

Target length: **55–60 seconds**. Record the real application at `http://localhost:8080` with the green Gnani status visible.

## Preparation

1. Click **Reset demo**.
2. Keep the browser zoom at 90–100%.
3. Put the voice panel and metrics on screen.
4. Use a quiet room, but include a small amount of natural background sound if recognition remains reliable.
5. Speak clearly and naturally; do not pretend a typed transcript came from speech.

## Script

### 0–6 seconds — problem

Narration:

> Small Indian shopkeepers should not need to stop serving customers just to type into inventory software.

Show the dashboard title and the Telugu + English badge.

### 6–18 seconds — stock query

Click the microphone and say:

> Basmati rice stock entha undi?

Show the Prisma transcript and the spoken stock response.

### 18–34 seconds — action, not chat

Say:

> Basmati rice rendu packets sale ayindi.

Point to the stock decreasing, revenue changing and the activity entry appearing.

Narration:

> The voice command performs a validated business transaction—it is not only a chatbot response.

### 34–47 seconds — intelligence

Say:

> Low stock products cheppu.

Then show the smart reorder panel with quantity, priority, days of stock cover and estimated cost.

### 47–57 seconds — business briefing

Say:

> How is my shop today?

Let Timbre speak the summary while the dashboard remains visible.

### 57–60 seconds — close

Narration:

> Vyapari Voice Copilot: speak, sell and restock with Indian-language AI.

End on the model badges and project name.

## Longer technical demo

For an interview or repository video, also show:

- API key is read from `.env`, not JavaScript
- browser WAV generation
- Prisma v3 with v2.5 fallback
- deterministic intent parsing and tests
- JSON persistence and undo
- CSV exports
- Docker deployment
