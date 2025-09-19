import assert from 'assert';
import { parseMask, applyMask } from '../src/lib/mask.js';

// parseMask
assert.deepStrictEqual(parseMask('*0'), { daily: false, monthly: null, yearly: false });
assert.deepStrictEqual(parseMask('00'), { daily: false, monthly: false, yearly: false });
assert.deepStrictEqual(parseMask('**'), { daily: false, monthly: null, yearly: null });

// applyMask
assert.deepStrictEqual(applyMask({ daily: false, monthly: true, yearly: false }, '*0'), { daily: false, monthly: true, yearly: false });
assert.deepStrictEqual(applyMask(undefined, '**'), { daily: false, monthly: false, yearly: false });
import { logger } from '../src/lib/logger.js';
logger.info('mask tests passed');
