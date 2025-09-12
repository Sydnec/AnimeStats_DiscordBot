import assert from 'assert';
import { computeStatsFromAniListData } from '../src/lib/stats.js';

// Build sample AniList-like data
const nowSec = Math.floor(Date.now() / 1000);
const oneDay = 24*60*60;
const data = {
  data: {
    MediaListCollection: {
      lists: [
        {
          entries: [
            // Anime A: progresse de 1 à 3 sur la même journée (devrait compter 2 épisodes)
            { media: { id: 1, title: { romaji: 'A' }, duration: 24 }, progress: 1, updatedAt: nowSec - 2*oneDay },
            { media: { id: 1, title: { romaji: 'A' }, duration: 24 }, progress: 3, updatedAt: nowSec - oneDay },
            // Anime B: progresse de 0 à 2 (devrait compter 2 épisodes)
            { media: { id: 2, title: { romaji: 'B' }, duration: 24 }, progress: 2, updatedAt: nowSec - oneDay },
          ]
        }
      ]
    }
  }
};

const startDate = new Date((nowSec - 3*oneDay) * 1000);
const endDate = new Date(nowSec * 1000);
const stats = computeStatsFromAniListData(data, startDate, endDate);
// Anime A: 1 (première entrée) + (3-1 = 2) (seconde entrée) = 3 épisodes
// Anime B: 2 (première entrée) = 2 épisodes
// Total attendu = 3 + 2 = 5
assert.strictEqual(stats.totalEpisodes, 5);
assert.strictEqual(stats.totalMinutes, 5*24);
import { logger } from '../src/lib/logger.js';
logger.info('stats tests passed');
