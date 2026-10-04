import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { ApiError } from "@/lib/api";

import { sessionKeys, signIn } from "../api";
import { SignInView, type SignInStatus } from "../components/SignInView";

/** SignInPage wires POST /v1/sessions to the sign-in view. */
export function SignInPage({ expired = false }: { expired?: boolean }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [email, setEmail] = useState("");
  const mutation = useMutation({
    mutationFn: (v: { email: string; password: string }) => signIn(v.email, v.password),
    onSuccess: (session) => {
      queryClient.setQueryData(sessionKeys.current, session);
      void navigate({ to: "/products" });
    },
  });

  const err = mutation.error;
  let status: SignInStatus = expired ? "expired" : "idle";
  if (mutation.isPending) status = "submitting";
  else if (err instanceof ApiError && err.status === 401) status = "wrong-credentials";
  else if (err instanceof ApiError && err.status === 429) status = "rate-limited";

  return (
    <SignInView
      status={status}
      email={email}
      {...(err instanceof ApiError && err.retryAfterSeconds !== undefined
        ? { retryAfterSeconds: err.retryAfterSeconds }
        : {})}
      onSubmit={(e, password) => {
        setEmail(e);
        mutation.mutate({ email: e, password });
      }}
    />
  );
}
