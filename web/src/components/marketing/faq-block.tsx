import { JsonLd, faqSchema } from "./json-ld";

export type FAQ = { question: string; answer: string };

type Props = {
  title?: string;
  eyebrow?: string;
  items: FAQ[];
  schemaId: string;
};

export function FAQBlock({
  title = "Frequently asked",
  eyebrow = "FAQ",
  items,
  schemaId,
}: Props) {
  return (
    <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
      <JsonLd id={schemaId} data={faqSchema(items)} />
      <div className="mx-auto max-w-[1100px]">
        <p className="mb-6 inline-flex items-center gap-2 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
          <span className="inline-block size-1 rounded-full bg-white/60" />
          {eyebrow}
        </p>
        <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.25rem,5vw,4rem)] max-w-[22ch]">
          {title}
        </h2>

        <dl className="mt-16 divide-y divide-white/[0.08] border-y border-white/[0.08]">
          {items.map((qa) => (
            <div key={qa.question} className="py-8">
              <dt className="font-[family-name:var(--font-display)] text-[18px] sm:text-[22px] leading-[1.3] tracking-[-0.01em] text-white/90">
                {qa.question}
              </dt>
              <dd className="mt-3 max-w-[68ch] text-[14px] sm:text-[15px] leading-[1.7] text-white/55">
                {qa.answer}
              </dd>
            </div>
          ))}
        </dl>
      </div>
    </section>
  );
}
