import { beforeEach, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import OnboardPage from "./page";
import DashboardPage from "../dashboard/page";
const state = vi.hoisted(() => ({ authenticated: true, session: { organization_memberships: [] as {role: string}[], workspace_memberships: [] }, unavailable: false }));
vi.mock("next/navigation", () => ({ redirect: (url: string) => { throw new Error(`redirect:${url}`); } }));
vi.mock("@/lib/auth/server", () => ({ getServerAuth: async () => state.authenticated ? { user: { id: "user" }, accessToken: "token" } : {}, toInitialAuth: () => ({}) }));
vi.mock("@workos-inc/authkit-nextjs", () => ({ withAuth: async () => state.authenticated ? { user: { id: "user" }, accessToken: "token" } : {} }));
vi.mock("@/app/providers", () => ({ AuthenticatedAppProviders: ({ children }: { children: React.ReactNode }) => children }));
vi.mock("./onboarding-wizard", () => ({ OnboardingWizard: ({ returnTo }: { returnTo?: string }) => <a href={returnTo}>Continue</a> }));
vi.mock("@/lib/api/server", () => ({ getServerApiClient: async () => ({ get: async () => { if (state.unavailable) throw new Error("offline"); return state.session; } }) }));
vi.mock("@/lib/api/client", () => ({ createApiClient: () => ({ get: async () => { if (state.unavailable) throw new Error("offline"); return state.session; } }) }));
const destination = "/tryouts?tryout=018f9f2e-65cb-7d0b-9c98-0df2ce4076d8";
const query = new URLSearchParams({ returnTo: destination });
beforeEach(() => { state.authenticated = true; state.unavailable = false; state.session.organization_memberships = []; });
it("passes selected result to onboarding wizard", async () => {
  expect(renderToStaticMarkup(await OnboardPage({ searchParams: Promise.resolve({ returnTo: destination }) }))).toContain(`href="${destination}"`);
});
it("retains intent when onboarding needs login", async () => {
  state.authenticated = false;
  await expect(OnboardPage({ searchParams: Promise.resolve({ returnTo: destination }) })).rejects.toThrow(`redirect:/auth/login?${query}`);
});
it("returns existing member to selected result", async () => {
  state.session.organization_memberships = [{ role: "org_admin" }];
  await expect(OnboardPage({ searchParams: Promise.resolve({ returnTo: destination }) })).rejects.toThrow(`redirect:${destination}`);
  await expect(DashboardPage({ searchParams: Promise.resolve({ returnTo: destination }) })).rejects.toThrow(`redirect:${destination}`);
});
it("carries dashboard intent into new account onboarding", async () => {
  await expect(DashboardPage({ searchParams: Promise.resolve({ returnTo: destination }) })).rejects.toThrow(`redirect:/onboard?${query}`);
});
it("preserves result intent in unavailable session retries", async () => {
  state.unavailable = true;
  expect(renderToStaticMarkup(await OnboardPage({ searchParams: Promise.resolve({ returnTo: destination }) }))).toContain(`/onboard?${query}`);
  expect(renderToStaticMarkup(await DashboardPage({ searchParams: Promise.resolve({ returnTo: destination }) }))).toContain(`/dashboard?${query}`);
});
