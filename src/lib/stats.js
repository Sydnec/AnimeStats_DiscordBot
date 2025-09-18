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
    // trier par createdAt asc afin de pouvoir suivre la progression par média
    const acts = activities.slice().sort((a, b) => (a.createdAt || 0) - (b.createdAt || 0));
    // map mediaId -> last known numeric progress (int)
    const lastProgress = new Map();

    // helper: find the last numeric progress for a given media id before a given timestamp (sec)
    function findPrevProgress(mediaId, beforeSec) {
      let best = null;
      const midStr = String(mediaId);
      // search in activities for entries older than beforeSec
      for (const a of activities) {
        if (!a || !a.media) continue;
        if (!a.createdAt) continue;
        if (a.createdAt >= beforeSec) continue;
        const aMid = String(a.media.id || 'noid');
        if (aMid !== midStr) continue;
        if (typeof a.progress === 'undefined' || a.progress === null) continue;
        const pStr = String(a.progress).trim().replace(/[–—]/g, '-');
        const rangeMatch = pStr.match(/(\d+)\D+(\d+)/);
        let val = null;
        if (rangeMatch) val = parseInt(rangeMatch[2], 10);
        else if (/^\d+$/.test(pStr)) val = parseInt(pStr, 10);
        if (val !== null) {
          if (!best || a.createdAt > best.ts) best = { val, ts: a.createdAt };
        }
      }

      if (best) return best.val;

      // fallback: search MediaListCollection snapshots if present
      const lists = (data && data.data && data.data.MediaListCollection && data.data.MediaListCollection.lists) || [];
      for (const l of lists) {
        if (!l || !Array.isArray(l.entries)) continue;
        for (const e of l.entries) {
          const id = e && e.media && e.media.id;
          if (typeof id === 'undefined') continue;
          if (String(id) !== midStr) continue;
          const updatedAt = e.updatedAt || 0;
          if (updatedAt >= beforeSec) continue;
          const prog = typeof e.progress === 'number' ? e.progress : parseInt(e.progress || '0', 10);
          if (!isNaN(prog)) {
            if (!best || updatedAt > best.ts) best = { val: prog, ts: updatedAt };
          }
        }
      }

      return best ? best.val : null;
    }

    acts.forEach((activity) => {
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

      // progress peut être "3", "3-4", etc. -> on calcule l'ajout comme delta depuis la dernière prog connue
      const mediaId = activity.media?.id || 'noid';
      // récupération du prev tel qu'on le connait jusqu'ici
      let prev = lastProgress.has(mediaId) ? lastProgress.get(mediaId) : null;
      let add = 0;
      const durationPerEp = (activity.media && activity.media.duration) || 24;

      if (typeof activity.progress !== 'undefined' && activity.progress !== null) {
        // Accept forms like "3", "1-3", "1 - 3", "1 – 3", etc.
        const progStr = String(activity.progress).trim().replace(/[–—]/g, '-');
        const rangeMatch = progStr.match(/(\d+)\D+(\d+)/);
        if (rangeMatch) {
          const start = parseInt(rangeMatch[1], 10);
          const end = parseInt(rangeMatch[2], 10);
          // si prev inconnu, tenter de le retrouver dans les anciennes activités/snapshots
          if (prev === null) {
            const found = findPrevProgress(mediaId, activity.createdAt);
            if (typeof found === 'number') prev = found;
            else prev = 0;
          }
          add = Math.max(0, end - prev);
          // mettre à jour lastProgress vers la valeur maximale connue
          lastProgress.set(mediaId, Math.max(prev || 0, end));
          logger.debug(`[ACTIVITY_PARSE] parsed range "${progStr}" -> potential ${end - start + 1} eps, delta vs prev(${prev}) = ${add}`);
        } else if (/^\d+$/.test(progStr)) {
          const val = parseInt(progStr, 10);
          if (prev === null) {
            const found = findPrevProgress(mediaId, activity.createdAt);
            if (typeof found === 'number') prev = found;
            else prev = 0;
          }
          add = Math.max(0, val - prev);
          lastProgress.set(mediaId, Math.max(prev || 0, val));
          logger.debug(`[ACTIVITY_PARSE] parsed single "${progStr}" -> val=${val}, delta vs prev(${prev}) = ${add}`);
        } else {
          logger.debug(`[ACTIVITY_PARSE] unknown progress format "${progStr}"`);
        }
      } else {
        // progress est null ou undefined -> probablement un changement de statut dans la liste utilisateur
        // Important: utiliser activity.status (statut dans la liste de l'utilisateur) et non media.status (statut de diffusion)
        const listStatus = activity.status ? String(activity.status).toUpperCase() : null;
        if (listStatus === 'COMPLETED') {
          // si l'utilisateur marque l'entrée comme COMPLETED, on peut compter le delta jusqu'à l'épisode total
          const totalEps = activity.media && typeof activity.media.episodes === 'number' ? activity.media.episodes : parseInt(activity.media && activity.media.episodes || '0', 10) || 0;
          if (prev === null) {
            const found = findPrevProgress(mediaId, activity.createdAt);
            if (typeof found === 'number') prev = found;
            else prev = 0;
          }
          add = Math.max(0, totalEps - prev);
          lastProgress.set(mediaId, Math.max(prev || 0, totalEps));
          logger.debug(`[ACTIVITY_PARSE] list status ${listStatus} -> add ${add} eps (total:${totalEps}, prev:${prev})`);
        } else {
          // pour tout autre changement de statut (DROPPED, PAUSED, PLANNING, etc.) ne rien compter
          logger.debug(`[ACTIVITY_PARSE] list status ${listStatus || 'UNKNOWN'} -> no eps counted for null progress`);
        }
      }

      // n'ajouter aux totals que si l'activité est dans la période et qu'il y a un delta positif
      if (date >= startDate && date <= endDate && add > 0) {
        const minutes = add * durationPerEp;
        totalMinutes += minutes;
        totalEpisodes += add;

        const title =
          activity.media?.title?.romaji ||
          activity.media?.title?.english ||
          activity.media?.title?.native ||
          "Titre inconnu";
        const day = date.toISOString().split("T")[0];
        dailyData[day] = (dailyData[day] || 0) + minutes;
        titleCounts[title] = (titleCounts[title] || 0) + add;
      }
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
