import Link from "next/link";
import { DEFAULT_MARKETING_NAV, type MarketingNavLink } from "@/lib/marketing-nav";

export function MarketingNavigation({ items = DEFAULT_MARKETING_NAV }: { items?: MarketingNavLink[] }) {
  const links = items.map(item => item.external
    ? <a key={item.href} href={item.href} target="_blank" rel="noopener noreferrer" className="block px-3 py-2 text-white/70 hover:text-white">{item.label}</a>
    : <Link key={item.href} href={item.href} className="block px-3 py-2 text-white/70 hover:text-white">{item.label}</Link>);
  return <>
    <div className="hidden lg:flex items-center">{links}</div>
    <details className="relative lg:hidden">
      <summary className="cursor-pointer px-3 py-2 text-white/80">Menu</summary>
      <div className="absolute right-0 top-full z-50 min-w-48 rounded-md border border-white/10 bg-[#101010] p-2 shadow-xl">{links}</div>
    </details>
  </>;
}
