export function computeStatsFromAniListData(data, startDate, endDate) {
  // reuse logic from index.js but simple and testable
  let totalMinutes = 0;
  let totalEpisodes = 0;
  const dailyData = {};
  const titleCounts = {};

  ((data.data && data.data.MediaListCollection && data.data.MediaListCollection.lists) || []).forEach(list => {
    (list.entries || []).forEach(entry => {
      if (!entry || !entry.updatedAt) return;
      const updatedAt = new Date(entry.updatedAt * 1000);
      if (updatedAt >= startDate && updatedAt <= endDate) {
        const epCount = entry.progress || 0;
        const durationPerEp = (entry.media && entry.media.duration) || 24;
        const minutes = epCount * durationPerEp;
        totalMinutes += minutes;
        totalEpisodes += epCount;
        const day = updatedAt.toISOString().split('T')[0];
        dailyData[day] = (dailyData[day] || 0) + minutes;
        const title = (entry.media && entry.media.title && (entry.media.title.romaji || entry.media.title.english || entry.media.title.native)) || 'Titre inconnu';
        titleCounts[title] = (titleCounts[title] || 0) + epCount;
      }
    });
  });

  let topDay = null;
  let topMinutes = 0;
  Object.entries(dailyData).forEach(([day, mins]) => {
    if (mins > topMinutes) { topMinutes = mins; topDay = day; }
  });

  const titles = Object.entries(titleCounts).map(([title, count]) => ({ title, count }));
  return { totalMinutes, totalEpisodes, dailyData, topDay, titles };
}

export default { computeStatsFromAniListData };
