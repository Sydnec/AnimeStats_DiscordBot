import assert from 'assert';
import { computeStatsFromAniListData } from '../src/lib/stats.js';

const nowSec = Math.floor(Date.now() / 1000);
const data = {
  data: {
    Page: {
      activities: [
        { createdAt: nowSec - 1000, progress: '1 - 3', media: { id: 20, title: { romaji: 'Range' }, duration: 24 } }
      ]
    }
  }
};

const startDate = new Date((nowSec - 3600) * 1000);
const endDate = new Date((nowSec + 3600) * 1000);
const stats = computeStatsFromAniListData(data, startDate, endDate);
// progress '1 - 3' doit compter comme 3 épisodes
assert.strictEqual(stats.totalEpisodes, 3);
assert.strictEqual(stats.totalMinutes, 3 * 24);
import { logger } from '../src/lib/logger.js';
logger.info('stats progress range test passed');
