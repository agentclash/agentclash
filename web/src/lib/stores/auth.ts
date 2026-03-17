import { create } from "zustand";
import { persist } from "zustand/middleware";

type DevAuth = {
  userId: string;
  email: string;
  displayName: string;
  workspaceMemberships: string; // format: "workspace-id:role,workspace-id:role"
};

type AuthState = {
  isAuthenticated: boolean;
  devAuth: DevAuth | null;
  activeWorkspaceId: string | null;
  login: (auth: DevAuth) => void;
  logout: () => void;
  setActiveWorkspace: (id: string) => void;
};

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      isAuthenticated: false,
      devAuth: null,
      activeWorkspaceId: null,

      login: (auth: DevAuth) => {
        // Store for the API client to pick up
        localStorage.setItem("agentclash_dev_auth", JSON.stringify(auth));
        // Extract first workspace ID as default
        const firstWorkspace = auth.workspaceMemberships.split(",")[0]?.split(":")[0] || null;
        set({
          isAuthenticated: true,
          devAuth: auth,
          activeWorkspaceId: get().activeWorkspaceId || firstWorkspace,
        });
      },

      logout: () => {
        localStorage.removeItem("agentclash_dev_auth");
        set({
          isAuthenticated: false,
          devAuth: null,
          activeWorkspaceId: null,
        });
      },

      setActiveWorkspace: (id: string) => {
        set({ activeWorkspaceId: id });
      },
    }),
    {
      name: "agentclash-auth",
      partialize: (state) => ({
        isAuthenticated: state.isAuthenticated,
        devAuth: state.devAuth,
        activeWorkspaceId: state.activeWorkspaceId,
      }),
    },
  ),
);
