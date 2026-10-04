import type { ScreenSpec } from "@/design/screen";

import { SignInView } from "../components/SignInView";

export const screen: ScreenSpec = {
  id: "S-02",
  name: "Sign in",
  feature: "auth",
  job: "Let a seeded seller or reviewer start a session, and say plainly why when they cannot.",
  states: {
    default: () => <SignInView status="idle" />,
    loading: () => <SignInView status="submitting" email="asha.reviewer@example.in" />,
    "wrong-credentials": () => (
      <SignInView status="wrong-credentials" email="asha.reviewer@example.in" />
    ),
    "rate-limited": () => (
      <SignInView status="rate-limited" email="asha.reviewer@example.in" retryAfterSeconds={540} />
    ),
    "session-expired": () => <SignInView status="expired" email="asha.reviewer@example.in" />,
  },
};
