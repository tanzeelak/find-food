# Find _your_ food
An AI-powered agent that finds specific menu items, not just restaurants, hat match your dietary restrictions. Most apps stop at "vegetarian-friendly." This goes further, asking "can I actually eat this, and is it nourishing?" by scouring individual menus item by item.

**Published:** [find-food on Codebuff](https://www.codebuff.com/publishers/tanzeela/agents/find-food/0.0.5) — 🥇 First place at the Codebuff Agent Builder Hackathon
![find-food screenshot](screenshot.webp)

## How it works

Two agents collaborate:

1. **`find-food`** — the orchestrator. Given a location and a list of dietary restrictions, it uses the [Exa](https://exa.ai) MCP to discover candidate restaurants nearby, then spawns a `research-restaurant` subagent for each one in parallel.

2. **`research-restaurant`** — the researcher. For a single restaurant, it uses Exa to check whether the menu has items that satisfy *all* of the specified dietary restrictions. Returns the restaurant name, matching menu items, a short vibe description, and a direct link to the menu.

## Requirements

- [Codebuff](https://codebuff.com) account + API key
- [Exa](https://exa.ai) API key (primary retrieval) or [Yelp API](https://docs.developer.yelp.com/) key (fallback)
- [OpenRouter](https://openrouter.ai) API key (for model selection)

## Setup

```bash
npm install
```

Create a `.env` file:

```
CODEBUFF_API_KEY=your_codebuff_api_key
EXA_API_KEY=your_exa_api_key
```

## Usage

```bash
codebuff find-food
```

When prompted, provide:
- **Location** — e.g. `"Mission District SF"` or `"Downtown Portland"`
- **Dietary restrictions** — e.g. `["gluten-free", "dairy-free", "pescatarian"]`

Each matching restaurant is returned with specific menu items, a vibe summary, and a link to its menu.

## What I learned

**Models need to translate fuzzy human intent.** Dietary restrictions are a good stress test for this: they're nuanced and overlapping. Getting a model to correctly interpret "I can't eat this" vs. "I prefer not to eat this" is non-trivial, and it matters a lot for utility.

**Retrieval quality matters more than tooling familiarity.** Exa outperformed Yelp significantly for this use case, even though Yelp is the obvious choice. Exa was harder to reason about as a human, but the results were better, which is a good reminder that the right tool isn't always the familiar one.

**Reliability is a moving target with evolving tools.** Exa ran into network issues, which forced a temporary swap to the [Yelp API](https://docs.developer.yelp.com/). Agentic systems need fallback strategies baked in.

**Model cost should scale with restriction complexity.** I used [OpenRouter](https://openrouter.ai) to route between models depending on the complexity of a user's restrictions. Someone with highly specific restrictions benefits from a more capable (and more expensive) model. Someone with simple preferences doesn't need that. A one-size-fits-all model choice leaves value on the table in both directions.

**Use only as much structure as you need.** [Codebuff](https://codebuff.com) is a structured agent builder. It makes it easy to define task types, spawn subagents, and share tools. Only some of that structure was genuinely useful here. Finding the minimum viable structure is its own design problem.

**Consistency is an open question.** Results varied noticeably across runs, even though restaurant menus are largely static. The source of that variability is worth digging into.

## Planned improvements

- Let users set dietary restrictions interactively
- Use Codebuff output schema for structured results
- Investigate result consistency across runs
- Auto switch models. Switch to Grok 4.3
- Limit options to 5
