import { LegalDocument } from "./LegalDocument";
import { PRIVACY_EFFECTIVE, PRIVACY_SECTIONS } from "./legalContent";

export function PrivacyPage() {
  return (
    <LegalDocument
      title="Privacy policy"
      intro="What Ownstall collects, why, and who else gets to see it."
      effective={PRIVACY_EFFECTIVE}
      sections={PRIVACY_SECTIONS}
    />
  );
}
