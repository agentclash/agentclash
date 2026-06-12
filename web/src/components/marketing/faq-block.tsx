import { JsonLd, faqSchema } from "./json-ld";

export type FAQ = { question: string; answer: string };

type Props = {
  title?: string;
  eyebrow?: string;
  items: FAQ[];
  schemaId: string;
  /** Use Geist sans for headings (required on /enterprise and new marketing pages). */
  sansHeadlines?: boolean;
};

export function FAQBlock({
  title = "Frequently asked",
  eyebrow = "FAQ",
  items,
  schemaId,
  sansHeadlines = false,
}: Props) {
  const sectionTitleClass = sansHeadlines
    ? "text-3xl font-sans font-semibold tracking-[-0.02em] text-white sm:text-4xl max-w-[22ch]"
    : "font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.25rem,5vw,4rem)] max-w-[22ch]";
  const questionClass = sansHeadlines
    ? "text-lg font-sans font-semibold tracking-[-0.01em] text-white/90 sm:text-xl"
    : "font-[family-name:var(--font-display)] text-lg sm:text-[22px] leading-[1.3] tracking-[-0.01em] text-white/90";
  return (
    <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
      <JsonLd id={schemaId} data={faqSchema(items)} />
      <div className="mx-auto max-w-[1100px]">
        <p className="mb-6 inline-flex items-center gap-2 text-2xs font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
          <span className="inline-block size-1 rounded-full bg-white/60" />
          {eyebrow}
        </p>
        <h2 className={sectionTitleClass}>{title}</h2>

        <dl className="mt-16 divide-y divide-white/[0.08] border-y border-white/[0.08]">
          {items.map((qa) => (
            <div key={qa.question} className="py-8">
              <dt className={questionClass}>{qa.question}</dt>
              <dd className="mt-3 max-w-[68ch] text-sm sm:text-base leading-[1.7] text-white/55">
                {qa.answer}
              </dd>
            </div>
          ))}
        </dl>
      </div>
    </section>
  );
}
