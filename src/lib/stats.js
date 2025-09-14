import { logger } from "./logger.js";

export function computeStatsFromAniListData(data, startDate, endDate) {
  let totalMinutes = 0;
  let totalEpisodes = 0;
  const dailyData = {};
  const titleCounts = {};

  // Case 1: AniList Activities (Page.activities)
  const activities =
    (data && data.data && data.data.Page && data.data.Page.activities) || [];
  if (activities.length > 0) {
    // Déduplication simple : ignorer les activités strictement identiques
    const seen = new Set();
    activities.forEach((activity) => {
      try {
        const key = `${activity.media?.id || 'noid'}|${String(activity.progress)}|${String(activity.createdAt)}`;
        if (seen.has(key)) return; // dédoublonner
        seen.add(key);
      } catch (e) {
        // ignore
      }
      logger.debug(
        `[ACTIVITY] ${
          activity.media?.title?.romaji ||
          activity.media?.title?.english ||
          activity.media?.title?.native ||
          "Titre inconnu"
        } | progress: ${activity.progress} | createdAt: ${
          activity.createdAt
        } | date: ${new Date(activity.createdAt * 1000).toISOString()}`
      );
      if (!activity || !activity.createdAt) return;
      const date = new Date(activity.createdAt * 1000);
      if (date < startDate || date > endDate) return;

      // progress peut être "3", "3-4", etc. -> on parse pour avoir le nombre d'ep
      let epCount = 1;
      if (typeof activity.progress !== 'undefined' && activity.progress !== null) {
        // Accept forms like "3", "1-3", "1 - 3", "1 – 3", etc.
        const progStr = String(activity.progress).trim().replace(/[–—]/g, '-');
        // Match two numbers separated by any non-digit chars (more tolerant than strict hyphen)
        const rangeMatch = progStr.match(/(\d+)\D+(\d+)/);
        if (rangeMatch) {
          const start = parseInt(rangeMatch[1], 10);
          const end = parseInt(rangeMatch[2], 10);
          epCount = Math.max(1, end - start + 1);
          logger.debug(`[ACTIVITY_PARSE] parsed range "${progStr}" -> ${epCount} eps (${start}-${end})`);
        } else if (/^\d+$/.test(progStr)) {
          // single integer progress corresponds to a single watched episode event
          epCount = 1;
          logger.debug(`[ACTIVITY_PARSE] parsed single "${progStr}" -> ${epCount} eps`);
        } else {
          logger.debug(`[ACTIVITY_PARSE] unknown progress format "${progStr}"`);
        }
      }

      const durationPerEp = (activity.media && activity.media.duration) || 24;
      const minutes = epCount * durationPerEp;

      totalMinutes += minutes;
      totalEpisodes += epCount;

      const day = date.toISOString().split("T")[0];
      dailyData[day] = (dailyData[day] || 0) + minutes;

      const title =
        activity.media?.title?.romaji ||
        activity.media?.title?.english ||
        activity.media?.title?.native ||
        "Titre inconnu";
      titleCounts[title] = (titleCounts[title] || 0) + epCount;
    });
  } else {
    // Case 2: MediaListCollection snapshot entries (older approach / tests)
    const lists =
      (data &&
        data.data &&
        data.data.MediaListCollection &&
        data.data.MediaListCollection.lists) ||
      [];
    // Flatten entries across lists
    const entries = [];
    for (const l of lists) {
      if (l && Array.isArray(l.entries)) {
        for (const e of l.entries) entries.push(e);
      }
    }

    // Group by media id
    const byMedia = new Map();
    for (const e of entries) {
      const id = e && e.media && e.media.id;
      if (typeof id === "undefined") continue;
      if (!byMedia.has(id)) byMedia.set(id, []);
      byMedia.get(id).push(e);
    }

    // For each media, sort by updatedAt asc and compute counts
    for (const [id, arr] of byMedia.entries()) {
      arr.sort((a, b) => (a.updatedAt || 0) - (b.updatedAt || 0));
      let prevProgress = null;
      let title = null;
      let durationPerEp = 24;
      for (const entry of arr) {
        const updatedAt = entry.updatedAt; // seconds
        if (!updatedAt) continue;
        const date = new Date(updatedAt * 1000);
        if (date < startDate || date > endDate) {
          // Even if outside range, we should keep prevProgress updated for later deltas
          prevProgress =
            typeof entry.progress === "number"
              ? entry.progress
              : parseInt(entry.progress || "0", 10) || prevProgress;
          if (!title)
            title =
              entry.media &&
              entry.media.title &&
              (entry.media.title.romaji ||
                entry.media.title.english ||
                entry.media.title.native);
          if (entry.media && entry.media.duration)
            durationPerEp = entry.media.duration;
          continue;
        }

        const progress =
          typeof entry.progress === "number"
            ? entry.progress
            : parseInt(entry.progress || "0", 10) || 0;
        if (title === null)
          title =
            (entry.media &&
              entry.media.title &&
              (entry.media.title.romaji ||
                entry.media.title.english ||
                entry.media.title.native)) ||
            "Titre inconnu";
        if (entry.media && entry.media.duration)
          durationPerEp = entry.media.duration;

        let add = 0;
        if (prevProgress === null) {
          // First snapshot we see in the period -> count the progress value itself
          add = progress;
        } else {
          add = Math.max(0, progress - prevProgress);
        }

        if (add > 0) {
          const minutes = add * durationPerEp;
          totalEpisodes += add;
          totalMinutes += minutes;
          const day = date.toISOString().split("T")[0];
          dailyData[day] = (dailyData[day] || 0) + minutes;
          titleCounts[title] = (titleCounts[title] || 0) + add;
        }

        prevProgress = progress;
      }
    }
  }

  let topDay = null;
  let topMinutes = 0;
  Object.entries(dailyData).forEach(([day, mins]) => {
    if (mins > topMinutes) {
      topMinutes = mins;
      topDay = day;
    }
  });

  const titles = Object.entries(titleCounts).map(([title, count]) => ({
    title,
    count,
  }));

  return { totalMinutes, totalEpisodes, dailyData, topDay, titles };
}

export default { computeStatsFromAniListData };
