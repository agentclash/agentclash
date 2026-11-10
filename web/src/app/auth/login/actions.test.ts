import { beforeEach, expect, it, vi } from "vitest";
import { signInAction, signUpAction } from "./actions";
import { readCallbackIntent } from "@/lib/auth/callback-intent";
const mocks = vi.hoisted(() => ({ signIn: vi.fn(), signUp: vi.fn(), set: vi.fn() }));
vi.mock("@workos-inc/authkit-nextjs", () => ({ getSignInUrl: mocks.signIn, getSignUpUrl: mocks.signUp }));
vi.mock("next/headers", () => ({ cookies: async () => ({ getAll: () => [], set: mocks.set }) }));
vi.mock("next/navigation", () => ({ redirect: (url: string) => { throw new Error(`redirect:${url}`); } }));
beforeEach(() => { vi.clearAllMocks(); vi.stubEnv("WORKOS_COOKIE_PASSWORD", "test-only-secret"); mocks.signIn.mockResolvedValue("https://auth.example/authorize?state=test-state"); mocks.signUp.mockResolvedValue("https://auth.example/authorize?state=test-state"); });
it.each([[signInAction, "signin"], [signUpAction, "signup"]] as const)("guards completion but retains original intent for retry (%s)", async (action, mode) => {
  const destination = "/tryouts?tryout=018f9f2e-65cb-7d0b-9c98-0df2ce4076d8";
  const form = new FormData(); form.set("returnTo", `${destination}&claim=1`);
  await expect(action(form)).rejects.toThrow("redirect:https://auth.example/authorize?state=test-state");
  expect(mode === "signin" ? mocks.signIn : mocks.signUp).toHaveBeenCalledWith({ returnTo: `/dashboard?${new URLSearchParams({ returnTo: destination })}` });
  expect(readCallbackIntent("test-state", mocks.set.mock.calls[0][1])).toEqual({ destination, mode });
});
