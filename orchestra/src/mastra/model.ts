import { createOpenRouter } from "@openrouter/ai-sdk-provider";
import { getEnv, requireEnv } from "./env.js";

const openrouter = createOpenRouter({ apiKey: requireEnv("OPENROUTER_API_KEY") });

export const defaultModelId = getEnv("LLM_MODEL", "anthropic/claude-sonnet-4");
export const researchModelId = getEnv("RESEARCH_MODEL", defaultModelId);

export const orchestratorModel = openrouter(defaultModelId);
export const researchModel = openrouter(researchModelId);
