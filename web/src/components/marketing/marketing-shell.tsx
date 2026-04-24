import type { ReactNode } from "react";
import { MarketingHeader } from "./marketing-header";
import { MarketingFooter } from "./marketing-footer";

type Props = {
  children: ReactNode;
};

export function MarketingShell({ children }: Props) {
  return (
    <main className="main min-h-screen flex flex-col">
      <MarketingHeader />
      {children}
      <MarketingFooter />
    </main>
  );
}
