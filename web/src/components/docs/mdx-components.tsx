import {
  Children,
  isValidElement,
  type ComponentPropsWithoutRef,
  type ReactNode,
} from "react";
import { Callout } from "@/components/docs/callout";
import { CopyableCodeBlock } from "@/components/docs/copyable-code-block";
import { slugify } from "@/lib/docs";
import { cn } from "@/lib/utils";

function flattenText(children: ReactNode): string {
  return Children.toArray(children)
    .map((child) => {
      if (typeof child === "string" || typeof child === "number") {
        return String(child);
      }

      if (isValidElement<{ children?: ReactNode }>(child)) {
        return flattenText(child.props.children);
      }

      return "";
    })
    .join(" ");
}

function DocsHeading({
  level,
  children,
  className,
  ...props
}: ComponentPropsWithoutRef<"h2"> & {
  level: 2 | 3;
}) {
  const id = slugify(flattenText(children));
  const Tag = level === 2 ? "h2" : "h3";

  return (
    <Tag
      id={id}
      {...props}
      className={cn(
        "scroll-mt-28 font-sans font-semibold tracking-tight text-zinc-100 not-italic antialiased",
        level === 2
          ? "mt-14 border-b border-zinc-800 pb-3 text-xl"
          : "mt-10 text-[1.0625rem] leading-snug",
        className,
      )}
    >
      {children}
    </Tag>
  );
}

export const docsMDXComponents = {
  Callout,
  h2: (props: ComponentPropsWithoutRef<"h2">) => (
    <DocsHeading level={2} {...props} />
  ),
  h3: (props: ComponentPropsWithoutRef<"h3">) => (
    <DocsHeading level={3} {...props} />
  ),
  pre: (props: ComponentPropsWithoutRef<"pre">) => (
    <CopyableCodeBlock>{props.children}</CopyableCodeBlock>
  ),
};
