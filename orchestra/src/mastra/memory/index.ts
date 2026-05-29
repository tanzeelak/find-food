import { Memory } from "@mastra/memory";
import { LibSQLStore } from "@mastra/libsql";
import { ensureFileUrlDir, getEnv } from "../env.js";

const memoryDbUrl = ensureFileUrlDir(getEnv("MEMORY_DB_URL", "file:./.mastra/find-food-memory.db"));

const userProfileTemplate = `# User Food Profile
- Dietary restrictions: 
- Food allergies: 
- Home / usual search location: 
- Food likes: 
- Food dislikes: 
`;

export const memory = new Memory({
  storage: new LibSQLStore({ id: "find-food-memory", url: memoryDbUrl }),
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
