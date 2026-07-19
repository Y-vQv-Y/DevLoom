export const defaultSkills = (import.meta.env.VITE_DEFAULT_SKILL_IDS || "")
  .split(",")
  .map((skillId: string) => skillId.trim())
  .filter(Boolean)
