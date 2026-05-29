# Codebuff Runtime

This folder contains the legacy Codebuff implementation of Find Food.

It is kept for reference while the project migrates to the Go backend in `../orchestra`.

## Contents

- `.agents/find-food.ts` - original orchestrator agent definition
- `.agents/research-restaurant.ts` - original per-restaurant researcher agent
- `codebuff.json` - Codebuff runtime config
- `main.ts` - Codebuff SDK usage example
- `.mcp.json` - local MCP config used by the Codebuff flow
- `architecture.md` - original Codebuff architecture notes

## Status

The current active runtime is the Go backend. Use this folder only to inspect or run the old Codebuff flow.
