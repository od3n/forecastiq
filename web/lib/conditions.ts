// Canonical weather-condition taxonomy v1 → display icon. Shared by the FvA
// observed-conditions table and the admin conditions timeline. The condition
// text belongs in tooltips/aria-labels, never as the only signal.
export const CONDITION_ICONS: Record<string, string> = {
  clear: "\u2600\uFE0F",            // ☀️
  partly_cloudy: "\u26C5",          // ⛅
  cloudy: "\u2601\uFE0F",           // ☁️
  fog: "\uD83C\uDF2B\uFE0F",        // 🌫️
  drizzle: "\uD83C\uDF26\uFE0F",    // 🌦️
  rain: "\uD83C\uDF27\uFE0F",       // 🌧️
  heavy_rain: "\uD83C\uDF27\uFE0F", // 🌧️
  thunderstorm: "\u26C8\uFE0F",     // ⛈️
  snow: "\u2744\uFE0F",             // ❄️
  sleet: "\uD83C\uDF28\uFE0F",      // 🌨️
  unknown: "\u2753",                // ❓
};

export function conditionIcon(code: string | null | undefined): string {
  return CONDITION_ICONS[code ?? "unknown"] ?? CONDITION_ICONS.unknown;
}

export function conditionLabel(code: string | null | undefined): string {
  return (code ?? "unknown").replace(/_/g, " ");
}
