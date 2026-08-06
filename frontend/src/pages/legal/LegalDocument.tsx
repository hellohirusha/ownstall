import { LegalSection } from "./legalContent";

interface LegalDocumentProps {
  title: string;
  intro: string;
  effective: string;
  sections: LegalSection[];
}

// Shared shell for /terms and /privacy so the two documents stay visually
// identical and only their content differs.
export function LegalDocument({
  title,
  intro,
  effective,
  sections,
}: LegalDocumentProps) {
  return (
    <>
      <section className="border-b border-ink-200 bg-ink-50">
        <div className="max-w-3xl mx-auto px-4 py-14">
          <h1 className="text-4xl font-bold text-ink-900 tracking-tight mb-3">
            {title}
          </h1>
          <p className="text-ink-500 mb-4">{intro}</p>
          <p className="text-sm text-ink-400">Effective {effective}</p>
        </div>
      </section>

      <section className="max-w-3xl mx-auto px-4 py-14">
        <div className="space-y-10">
          {sections.map((section) => (
            <div key={section.heading}>
              <h2 className="text-lg font-bold text-ink-900 mb-3">
                {section.heading}
              </h2>
              <div className="space-y-3">
                {section.body.map((paragraph, i) => (
                  <p key={i} className="text-ink-600 leading-relaxed">
                    {paragraph}
                  </p>
                ))}
              </div>
            </div>
          ))}
        </div>
      </section>
    </>
  );
}
