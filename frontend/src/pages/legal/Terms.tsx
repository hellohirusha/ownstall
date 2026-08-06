import { LegalDocument } from "./LegalDocument";
import { TERMS_EFFECTIVE, TERMS_SECTIONS } from "./legalContent";

export function TermsPage() {
  return (
    <LegalDocument
      title="Terms of service"
      intro="The rules for opening a stall on Ownstall and for buying from one."
      effective={TERMS_EFFECTIVE}
      sections={TERMS_SECTIONS}
    />
  );
}
