import { Skeleton } from "@/components/ui/skeleton";

export function WorkspaceListLoading({
  showTabs = false,
  rows = 5,
  actionCount = 1,
}: {
  showTabs?: boolean;
  rows?: number;
  actionCount?: number;
}) {
  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-2">
          <Skeleton className="h-7 w-44" />
          <Skeleton className="h-4 w-80 max-w-full" />
        </div>
        <div className="flex items-center gap-2">
          {Array.from({ length: actionCount }, (_, index) => (
            <Skeleton key={index} className="h-9 w-28" />
          ))}
        </div>
      </div>

      {showTabs ? (
        <div className="flex items-center gap-4 border-b border-border pb-2">
          <Skeleton className="h-5 w-16" />
          <Skeleton className="h-5 w-24" />
        </div>
      ) : null}

      <div className="rounded-lg border border-border">
        <div className="space-y-3 p-4">
          {Array.from({ length: rows }, (_, index) => (
            <div
              key={index}
              className="grid grid-cols-[minmax(0,2fr)_repeat(3,minmax(0,1fr))] gap-4"
            >
              <Skeleton className="h-5 w-full" />
              <Skeleton className="h-5 w-full" />
              <Skeleton className="h-5 w-full" />
              <Skeleton className="h-5 w-full" />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

export function WorkspaceDetailLoading() {
  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <Skeleton className="h-4 w-20" />
        <Skeleton className="h-8 w-56" />
        <Skeleton className="h-4 w-[32rem] max-w-full" />
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        {Array.from({ length: 3 }, (_, index) => (
          <div key={index} className="rounded-lg border border-border p-4">
            <Skeleton className="h-4 w-24" />
            <Skeleton className="mt-3 h-8 w-20" />
            <Skeleton className="mt-2 h-4 w-32" />
          </div>
        ))}
      </div>

      <div className="rounded-lg border border-border p-4">
        <div className="space-y-3">
          {Array.from({ length: 6 }, (_, index) => (
            <Skeleton key={index} className="h-5 w-full" />
          ))}
        </div>
      </div>
    </div>
  );
}
