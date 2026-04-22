import {
  Children,
  isValidElement,
  type ComponentPropsWithoutRef,
  type ReactNode,
} from "react";
import { Callout } from "@/components/docs/callout";
import { slugify } from "@/lib/docs";

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
  ...props
}: ComponentPropsWithoutRef<"h2"> & {
  level: 2 | 3;
}) {
  const id = slugify(flattenText(children));
  const Tag = level === 2 ? "h2" : "h3";

  return (
    <Tag id={id} {...props}>
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
};
