// "Mon 28 Sep" in every language: the day before the month, never the US "Mon, Sep 28"
export function fmtDayShort(d: Date, locale?: string): string {
  const weekday = d.toLocaleDateString(locale, { weekday: "short" });
  const month = d.toLocaleDateString(locale, { month: "short" });
  return `${weekday} ${d.getDate()} ${month}`;
}

// calendar days from `now` to `d`: 0 today, 1 tomorrow
export function dayOffset(d: Date, now: Date): number {
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const that = new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  return Math.round((that - today) / 86400000);
}
