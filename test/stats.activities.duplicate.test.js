import assert from 'assert';
import { computeStatsFromAniListData } from '../src/lib/stats.js';

// Simule deux activités identiques (mêmes media.id, progress, createdAt)
const nowSec = Math.floor(Date.now() / 1000);
const data = {
  data: {
    Page: {
      activities: [
        { createdAt: nowSec - 1000, progress: 3, media: { id: 10, title: { romaji: 'Dup' }, duration: 24 } },
        { createdAt: nowSec - 1000, progress: 3, media: { id: 10, title: { romaji: 'Dup' }, duration: 24 } }
      ]
    }
  }
};

const startDate = new Date((nowSec - 3600) * 1000);
const endDate = new Date((nowSec + 3600) * 1000);
const stats = computeStatsFromAniListData(data, startDate, endDate);
// progress '3' correspond à 1 épisode par activité. Sans dédoublonnage, totalEpisodes serait 2 (1+1). Avec dédoublonnage, il doit être 1.
assert.strictEqual(stats.totalEpisodes, 1);
assert.strictEqual(stats.totalMinutes, 1 * 24);
import { logger } from '../src/lib/logger.js';
logger.info('stats duplicate activities test passed');
