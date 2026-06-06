import { Mastra } from "@mastra/core/mastra";
import { MastraCompositeStore } from "@mastra/core/storage";
import { LibSQLStore } from "@mastra/libsql";
import { DuckDBStore } from "@mastra/duckdb";
import { Observability, MastraStorageExporter } from "@mastra/observability";
import { resolveLibSQLConnection, resolveDataPath } from "./env.js";
import { findFoodAgent } from "./agents/find-food.js";
import { researchRestaurantAgent } from "./agents/research-restaurant.js";

// LibSQL cannot persist observability metrics (only traces), so the
// observability domain is routed to DuckDB, an OLAP store that supports them.
export const mastra = new Mastra({
  agents: {
    findFood: findFoodAgent,
    researchRestaurant: researchRestaurantAgent
  },
  storage: new MastraCompositeStore({
    id: "find-food-storage",
    default: new LibSQLStore({
      id: "find-food-mastra",
      ...resolveLibSQLConnection(
        "MASTRA_DB_URL",
        "MASTRA_DB_AUTH_TOKEN",
        `file:${resolveDataPath(".mastra/mastra.db")}`
      )
    }),
    domains: {
      observability: new DuckDBStore({
        path: resolveDataPath(".mastra/find-food-observability.duckdb")
      }).observability
    }
  }),
  observability: new Observability({
    configs: {
      default: {
        serviceName: "find-food",
        exporters: [new MastraStorageExporter()]
      }
    }
  })
});
