// Hues avoid green, amber and red, which are reserved for blood-pressure levels. Every main
// colour keeps white text above 4.5:1.
const PALETTE = [
  { main: '#3a5a99', tint: '#eaeef7' },
  { main: '#16716f', tint: '#e5f2f1' },
  { main: '#7a4a9e', tint: '#f1ebf6' },
  { main: '#a8396a', tint: '#f7e9ef' },
  { main: '#2f6a8f', tint: '#e6f0f6' },
  { main: '#4b6272', tint: '#ebeff2' },
] as const

export type PersonColor = (typeof PALETTE)[number]

/** Derived from the profile id so a person has the same colour on every family member's phone. */
export function personColor(profileId: number): PersonColor {
  const n = PALETTE.length
  return PALETTE[(((profileId - 1) % n) + n) % n]
}

export function personStyle(profileId: number): Record<string, string> {
  const c = personColor(profileId)
  return { '--person': c.main, '--person-tint': c.tint }
}

export function initialOf(name: string): string {
  return Array.from(name.trim())[0] ?? '?'
}
