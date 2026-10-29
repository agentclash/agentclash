"use client";
import { useEffect, useRef, useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { dollars, vibeFetch } from "@/lib/vibe";
type Credits = {
  under_review: boolean;
  balance_nano_usd: number;
  held_nano_usd: number;
  available_nano_usd: number;
  products: {
    id: string;
    credits_nano_usd: number;
    price_minor: number;
    currency: string;
  }[];
};
const creditsPath = (workspace: string) => `/credits?workspace_id=${encodeURIComponent(workspace)}`;
// One controlled instance at the page root; openers close their own dialog first.
export function CreditsDialog({ workspace, open, onOpenChange }: { workspace: string; open: boolean; onOpenChange: (open: boolean) => void }) {
  const { getAccessToken } = useAccessToken();
  const token = useRef(getAccessToken);
  useEffect(() => { token.current = getAccessToken; }, [getAccessToken]);
  const [credits, setCredits] = useState<Credits | null>(null);
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  async function load() {
    setError("");
    try {
      setCredits(await vibeFetch<Credits>(creditsPath(workspace), await token.current()));
    } catch (e) {
      setError((e as Error).message);
    }
  }
  useEffect(() => {
    if (!open) return;
    let live = true;
    // A failure from an earlier opening must not show while this request runs.
    setError("");
    token.current().then(auth => vibeFetch<Credits>(creditsPath(workspace), auth)).then(
      (next) => { if (live) { setCredits(next); setError(""); } },
      (e) => { if (live) setError((e as Error).message); },
    );
    return () => { live = false; };
  }, [open, workspace]);
  async function checkout(product: string) {
    setPending(true);
    setError("");
    try {
      const key = `vibe-credit-checkout:${workspace}:${product}`;
      const id = localStorage.getItem(key) || crypto.randomUUID();
      localStorage.setItem(key, id);
      const result = await vibeFetch<{
        state: string;
        checkout_url: string | null;
      }>("/credit-checkouts", await getAccessToken(), {
        method: "POST",
        body: JSON.stringify({
          id,
          workspace_id: workspace,
          product_id: product,
        }),
      });
      if (result.state === "PAID") {
        localStorage.removeItem(key);
        await load();
      } else if (result.checkout_url)
        window.location.assign(result.checkout_url);
      else
        setError(
          "This checkout is awaiting confirmation from the payment provider. It will not be automatically repeated.",
        );
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setPending(false);
    }
  }
  return (
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="vibe-workspace vibe-dialog">
          <DialogTitle>Workspace AI credits</DialogTitle>
          <DialogDescription>
            Operations reserve their maximum cost first. Actual provider spend
            is settled after execution. Uncertain calls stay reserved while
            accounting is resolved.
          </DialogDescription>
          <div className="vibe-dialog-body space-y-4">
          {credits && (
            <>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <p className="text-2xl font-semibold tabular-nums">
                    {dollars(credits.available_nano_usd)}
                  </p>
                  <p className="text-xs text-muted-foreground">Available</p>
                </div>
                <div>
                  <p className="text-2xl font-semibold tabular-nums">
                    {dollars(credits.held_nano_usd)}
                  </p>
                  <p className="text-xs text-muted-foreground">Reserved</p>
                </div>
              </div>
              {credits.under_review && (
                <p role="alert" className="text-sm text-builder-warn">
                  This account is under review. Hosted calls and credit
                  purchases are paused.
                </p>
              )}
              <div className="grid gap-2">
              {credits.products.map((p) => (
                <Button
                  key={p.id}
                  variant="outline"
                  disabled={pending || credits.under_review}
                  onClick={() => checkout(p.id)}
                >
                  Add {dollars(p.credits_nano_usd)} credits · $
                  {(p.price_minor / 100).toFixed(2)} before tax
                </Button>
              ))}
              </div>
              <p className="text-sm text-muted-foreground">
                Only an organization administrator can purchase top-ups. Payment
                takes place on Dodo’s checkout page.
              </p>
            </>
          )}
          {error && (
            <p role="alert" className="text-sm text-builder-warn">
              {error}
            </p>
          )}
          </div>
        </DialogContent>
      </Dialog>
  );
}
