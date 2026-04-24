"use client";

import { useEffect, useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { Copy, Loader2, Share2 } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { createApiClient } from "@/lib/api/client";
import type {
  CreatePublicShareLinkResponse,
  PublicShareResourceType,
} from "@/lib/api/types";

interface CreatePublicShareButtonProps {
  resourceType: PublicShareResourceType;
  resourceId: string;
  label?: string;
  size?: "xs" | "sm" | "default";
  variant?: "outline" | "ghost" | "secondary" | "default";
  disabled?: boolean;
}

export function CreatePublicShareButton({
  resourceType,
  resourceId,
  label = "Share",
  size = "sm",
  variant = "outline",
  disabled,
}: CreatePublicShareButtonProps) {
  const { getAccessToken } = useAccessToken();
  const [sharing, setSharing] = useState(false);
  const [url, setUrl] = useState<string | null>(null);

  useEffect(() => {
    setUrl(null);
  }, [resourceType, resourceId]);

  async function handleShare() {
    if (url) {
      await copyURL(url);
      return;
    }

    setSharing(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const result = await api.post<CreatePublicShareLinkResponse>(
        "/v1/share-links",
        {
          resource_type: resourceType,
          resource_id: resourceId,
        },
      );
      setUrl(result.url);
      await copyURL(result.url);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to create share link",
      );
    } finally {
      setSharing(false);
    }
  }

  return (
    <Button
      type="button"
      variant={variant}
      size={size}
      onClick={handleShare}
      disabled={disabled || sharing}
      title={url ? "Copy public link" : "Create public read-only link"}
    >
      {sharing ? (
        <Loader2 className="size-3.5 animate-spin" />
      ) : url ? (
        <Copy className="size-3.5" />
      ) : (
        <Share2 className="size-3.5" />
      )}
      {url ? "Copy link" : label}
    </Button>
  );
}

async function copyURL(url: string) {
  try {
    await navigator.clipboard.writeText(url);
    toast.success("Public link copied");
  } catch {
    toast.success("Public link created", {
      description: url,
    });
  }
}
