import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import HomePage from "./landing";

export default async function RootPage() {
  const { user } = await withAuth();
  if (user) redirect("/dashboard");
  return <HomePage />;
}
