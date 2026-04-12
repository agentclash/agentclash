"use client";

import { useAuth } from "@workos-inc/authkit-nextjs/components";
import { Button } from "@/components/ui/button";

export function SignOutButton() {
  const { signOut } = useAuth();

  return (
    <Button variant="outline" size="sm" onClick={() => signOut()}>
      Sign out
    </Button>
  );
}
