import type { ScreenSpec } from "@/design/screen";
import { AppFrame } from "@/features/shell/components/AppFrame";
import { budgetOk, reviewer } from "@/features/shell/fixtures";

import { ChannelsView, type ChannelRow, type ChannelsViewProps } from "../components/ChannelsView";

// Rule values are the team's draft for sign-off (Q-003), not marketplace facts.
const amazon: ChannelRow = {
  id: "amazon_style",
  name: "Amazon-style",
  enabled: true,
  loadError: null,
  titleMaxLength: 200,
  requiredAttributes: ["colour", "fit"],
  bannedWordCount: 24,
  lastRecheck: { listingsRechecked: 297, approvalsCleared: 4, when: "4 Oct 2026, 12:30 pm IST" },
};

const website: ChannelRow = {
  id: "own_website",
  name: "Own website",
  enabled: true,
  loadError: null,
  titleMaxLength: 120,
  requiredAttributes: ["colour"],
  bannedWordCount: 6,
  lastRecheck: null,
};

const flipkart: ChannelRow = {
  id: "flipkart_style",
  name: "Flipkart-style",
  enabled: false,
  loadError: "title_max_length must be a whole number (line 4)",
  titleMaxLength: null,
  requiredAttributes: [],
  bannedWordCount: 0,
  lastRecheck: null,
};

const framed = (props: ChannelsViewProps) => (
  <AppFrame current="channels" {...reviewer} budget={budgetOk}>
    <ChannelsView {...props} />
  </AppFrame>
);

export const screen: ScreenSpec = {
  id: "S-07",
  name: "Channels",
  feature: "channels",
  job: "Show the rules each channel checks listings against, and say when a channel is off and why.",
  states: {
    loading: () => framed({ status: "loading" }),
    success: () => framed({ status: "ready", channels: [amazon, website] }),
    "channel-disabled": () => framed({ status: "ready", channels: [amazon, website, flipkart] }),
    error: () => framed({ status: "error", requestId: "req_01J9Z6" }),
  },
};
