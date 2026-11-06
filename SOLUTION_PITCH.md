# Vyapari Voice Copilot

## One-Page Solution Pitch

### Problem

Many small Indian retailers still manage inventory and sales using notebooks, memory, or software that requires frequent typing and menu navigation. This becomes difficult while serving customers, especially for users who naturally speak in regional languages mixed with English product names.

### Solution

Vyapari Voice Copilot is a Telugu-English voice operations assistant for small retailers. It allows shop owners to perform common business tasks through natural speech.

For example, a user can say:

> “Basmati rice rendu packets sale ayindi.”

Gnani Prisma converts the speech to text. A Go backend then identifies the intent, product, and quantity, validates the request, updates inventory and sales, recalculates revenue, profit, stock levels, and reorder suggestions, and stores the new state persistently. Gnani Timbre then speaks the result back in Telugu or English.

### Key Features

- Telugu-English code-mixed voice commands
- Voice-based stock checking
- Voice-based sale recording
- Voice-based restocking
- Low-stock alerts
- Daily revenue and profit analytics
- Best-selling product insights
- Demand-aware reorder recommendations
- Persistent inventory and transaction data
- Undo support and traceable activity history
- Inventory and reorder CSV exports
- Secure server-side API key handling

### Gnani AI Usage

- **Gnani Prisma** for multilingual speech-to-text
- **Gnani Timbre** for Telugu and English text-to-speech

### What Makes It Different

Vyapari is not just a voice chatbot. A spoken command performs a real, validated, persistent business operation. The change is immediately reflected in inventory, sales metrics, analytics, and activity history, while the system also provides a spoken response.

### Target Users

- Kirana and neighborhood store owners
- Small wholesalers
- Family-run retail shops
- Regional-language users who prefer speaking over typing

### Technology Stack

- Go backend
- HTML, CSS, JavaScript frontend
- Gnani Prisma
- Gnani Timbre
- Web Audio API
- Local persistent JSON storage

### Impact

Vyapari Voice Copilot reduces the effort required to use digital retail software and makes shop management more accessible to users who are more comfortable speaking than typing.