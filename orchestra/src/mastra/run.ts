import { RequestContext } from "@mastra/core/di";
import { mastra } from "./index.js";

export type TurnInput = {
  message: string;
  resourceId: string;
  threadId: string;
};

export async function runFindFoodTurn({ message, resourceId, threadId }: TurnInput) {
  const requestContext = new RequestContext();
  requestContext.set("resourceId", resourceId);

  return mastra.getAgent("findFood").stream(message, {
    memory: { resource: resourceId, thread: threadId },
    requestContext
  });
}
