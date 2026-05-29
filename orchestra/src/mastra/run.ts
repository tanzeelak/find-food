import { findFoodAgent } from "./agents/find-food.js";

export type TurnInput = {
  message: string;
  resourceId: string;
  threadId: string;
};

export async function runFindFoodTurn({ message, resourceId, threadId }: TurnInput) {
  return findFoodAgent.stream(message, {
    memory: { resource: resourceId, thread: threadId }
  });
}
