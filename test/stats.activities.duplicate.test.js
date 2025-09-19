import assert from 'assert';
import { computeStatsFromAniListData } from '../src/lib/stats.js';

// Simule deux activités identiques (mêmes media.id, progress, createdAt)
// Ajout de champs réalistes : status, media.episodes, media.duration
const nowSec = Math.floor(Date.now() / 1000);
const data = {
  data: {
    Page: {
      activities: [
        {
          createdAt: nowSec - 1000,
          progress: 3,
          status: 'CURRENT',
          media: {
            id: 10,
            title: { romaji: 'Dup' },
            duration: 24,
            episodes: 12
          }
        },
        {
          createdAt: nowSec - 1000,
          progress: 3,
          status: 'CURRENT',
          media: {
            id: 10,
            title: { romaji: 'Dup' },
            duration: 24,
            episodes: 12
          }
        }
      ]
    }
  }
};

const startDate = new Date((nowSec - 3600) * 1000);
const endDate = new Date((nowSec + 3600) * 1000);
const stats = computeStatsFromAniListData(data, startDate, endDate);
console.log(stats);
// Le code traite `progress` comme valeur absolue et, en l'absence de prev connu,
// compte la différence vs 0. Ici progress=3 -> 3 épisodes comptés.
assert.strictEqual(stats.totalEpisodes, 3);
assert.strictEqual(stats.totalMinutes, 3 * 24);
import { logger } from '../src/lib/logger.js';
logger.info('stats duplicate activities test passed');
