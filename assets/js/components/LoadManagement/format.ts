// "Mon 28 Sep" in every language: the day before the month, never the US "Mon, Sep 28"
export function fmtDayShort(d: Date, locale?: string): string {
  const weekday = d.toLocaleDateString(locale, { weekday: "short" });
  const month = d.toLocaleDateString(locale, { month: "short" });
  return `${weekday} ${d.getDate()} ${month}`;
}
