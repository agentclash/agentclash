import type { ReactNode } from "react";
import { MarketingHeader } from "./marketing-header";
import { MarketingFooter } from "./marketing-footer";

type Props = {
  children: ReactNode;
  showFooter?: boolean;
  /** Optional promo banner rendered above the header (top of page). */
  banner?: ReactNode;
};

export function MarketingShell({ children, showFooter = true, banner }: Props) {
  return (
    <main className="main min-h-screen flex flex-col">
      {banner}
      <MarketingHeader />
      {children}
      {showFooter ? <MarketingFooter /> : null}
    </main>
  );
}
