import type { ReactNode } from "react";
import { MarketingHeader } from "./marketing-header";
import { MarketingFooter } from "./marketing-footer";

type Props = {
  children: ReactNode;
  showFooter?: boolean;
};

export function MarketingShell({ children, showFooter = true }: Props) {
  return (
    <main className="main min-h-screen flex flex-col">
      <MarketingHeader />
      {children}
      {showFooter ? <MarketingFooter /> : null}
    </main>
  );
}
