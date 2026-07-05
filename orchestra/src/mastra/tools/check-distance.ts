import { createTool } from "@mastra/core/tools";
import { z } from "zod";
import { getEnv } from "../env.js";

async function geocode(query: string): Promise<{ lat: number; lon: number; place_name: string } | null> {
  const token = getEnv("MAPBOX_TOKEN", "");
  const url = `https://api.mapbox.com/geocoding/v5/mapbox.places/${encodeURIComponent(query)}.json?access_token=${token}&limit=1`;
  const res = await fetch(url);
  const data = await res.json() as { features?: Array<{ center: [number, number]; place_name: string }> };
  const feature = data.features?.[0];
  if (!feature) return null;
  return { lon: feature.center[0], lat: feature.center[1], place_name: feature.place_name };
}

async function getWalkingMinutes(
  from: { lat: number; lon: number },
  to: { lat: number; lon: number }
): Promise<number | null> {
  const token = getEnv("MAPBOX_TOKEN", "");
  const url = `https://api.mapbox.com/directions/v5/mapbox/walking/${from.lon},${from.lat};${to.lon},${to.lat}?access_token=${token}&overview=false`;
  const res = await fetch(url);
  const data = await res.json() as { routes?: Array<{ duration: number }> };
  const duration = data.routes?.[0]?.duration;
  return duration != null ? Math.round(duration / 60) : null;
}

export const checkDistanceTool = createTool({
  id: "check-distance",
  description:
    "Verifies walking time between the user's location and a restaurant using Mapbox. Use this after researching a restaurant to confirm it is within the user's requested distance. If no max distance was specified, use 10 minutes as the default.",
  inputSchema: z.object({
    restaurantName: z.string(),
    restaurantAddress: z.string().describe("Restaurant name + city, e.g. 'Mission Chinese Food San Francisco CA'"),
    userLocation: z.string().describe("User's location, e.g. 'Mission District San Francisco' or a street address"),
    maxWalkingMinutes: z.number().default(10).describe("Maximum acceptable walking time in minutes"),
  }),
  outputSchema: z.object({
    withinThreshold: z.boolean(),
    walkingMinutes: z.number().nullable(),
    restaurantPlaceName: z.string().nullable(),
    userPlaceName: z.string().nullable(),
  }),
  execute: async (input) => {
    const [restaurantGeo, userGeo] = await Promise.all([
      geocode(input.restaurantAddress),
      geocode(input.userLocation),
    ]);

    if (!restaurantGeo || !userGeo) {
      // Can't verify — don't filter out the restaurant
      return {
        withinThreshold: true,
        walkingMinutes: null,
        restaurantPlaceName: restaurantGeo?.place_name ?? null,
        userPlaceName: userGeo?.place_name ?? null,
      };
    }

    const minutes = await getWalkingMinutes(userGeo, restaurantGeo);

    return {
      withinThreshold: minutes == null || minutes <= input.maxWalkingMinutes,
      walkingMinutes: minutes,
      restaurantPlaceName: restaurantGeo.place_name,
      userPlaceName: userGeo.place_name,
    };
  },
});
