export * from "./signer";
export * from "./types";
export * from "./constants";
export { ExactMultiversXScheme } from "./exact/client/scheme";
export { ExactMultiversXFacilitator } from "./exact/facilitator/scheme";
export { ExactMultiversXServer } from "./exact/server/scheme";

import { registerScheme } from "@x402/core";
import { ExactMultiversXScheme } from "./exact/client/scheme";
import { ExactMultiversXFacilitator } from "./exact/facilitator/scheme";
import { ExactMultiversXServer } from "./exact/server/scheme";

export function registerYourChainScheme() {
    registerScheme("multiversx-exact-v1", {
        client: ExactMultiversXScheme,
        facilitator: ExactMultiversXFacilitator,
        server: ExactMultiversXServer
    });
}
