import { Memory } from "@mastra/memory";
import { LibSQLStore } from "@mastra/libsql";
import { resolveLibSQLConnection, resolveDataPath } from "../env.js";

const memoryConnection = resolveLibSQLConnection(
  "MEMORY_DB_URL",
  "MEMORY_DB_AUTH_TOKEN",
  `file:${resolveDataPath(".mastra/find-food-memory.db")}`,
  { urlKey: "MASTRA_DB_URL", tokenKey: "MASTRA_DB_AUTH_TOKEN" }
);

const userProfileTemplate = `# User Food Profile
- Dietary restrictions: 
- Food allergies: 
- Home / usual search location: 
- Food likes: 
- Food dislikes: 
`;

export const memory = new Memory({
  storage: new LibSQLStore({ id: "find-food-memory", ...memoryConnection }),
  options: {
    lastMessages: 20,
    semanticRecall: false,
    workingMemory: {
      enabled: true,
      scope: "resource",
      template: userProfileTemplate
    }
  }
});
