import { Mastra } from "@mastra/core/mastra";
import { LibSQLStore } from "@mastra/libsql";
import { ensureFileUrlDir, getEnv } from "./env.js";
import { findFoodAgent } from "./agents/find-food.js";
import { researchRestaurantAgent } from "./agents/research-restaurant.js";

export const mastra = new Mastra({
  agents: {
    findFood: findFoodAgent,
    researchRestaurant: researchRestaurantAgent
  },
  storage: new LibSQLStore({
    id: "find-food-mastra",
    url: ensureFileUrlDir(getEnv("MASTRA_DB_URL", "file:./.mastra/mastra.db"))
  })
});
