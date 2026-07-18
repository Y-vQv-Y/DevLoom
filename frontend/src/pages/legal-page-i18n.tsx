import type { TFunction } from "i18next";
import type { ReactNode } from "react";

import type { LegalSection } from "@/components/welcome/legal-terminal-page";
import { BRAND } from "@/config/brand";

export type LegalPageCopy = {
  eyebrow: string;
  title: string;
  subtitle: string;
  tags: string[];
  sections: Array<Omit<LegalSection, "footer">>;
};

export function withContactFooter(
  sections: Array<Omit<LegalSection, "footer">>,
  footer: ReactNode
): LegalSection[] {
  return sections.map((section) => (section.id === "contact" ? { ...section, footer } : section));
}

export function renderOfficialChannels(t: TFunction, keyPrefix: string, _isGlobalRegion: boolean) {
  return (
    <>
      {t(`${keyPrefix}.prefix`)}
      <a className="text-[var(--a-accent)] hover:underline" href={BRAND.repositoryUrl} target="_blank" rel="noreferrer">
        {t(`${keyPrefix}.repository`)}
      </a>
      {t(`${keyPrefix}.or`)}
      <a className="text-[var(--a-accent)] hover:underline" href={BRAND.supportUrl} target="_blank" rel="noreferrer">
        {t(`${keyPrefix}.support`)}
      </a>
      {t(`${keyPrefix}.suffix`)}
    </>
  );
}
