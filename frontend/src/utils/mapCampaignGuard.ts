const GUARD_BEGIN = '// @l4d2-manager:campaign-guard:v1 begin';
const CODES_PREFIX = '// @l4d2-manager:campaign-codes:v1 ';
const CONTENT_BEGIN = '// @l4d2-manager:campaign-content:v1 begin';
const CONTENT_END = '// @l4d2-manager:campaign-content:v1 end';
const GUARD_END = '// @l4d2-manager:campaign-guard:v1 end';

const MAP_CODE_PATTERN = /^[a-zA-Z0-9_-]+$/;
const MANAGED_MARKERS = [
  GUARD_BEGIN,
  CODES_PREFIX,
  CONTENT_BEGIN,
  CONTENT_END,
  GUARD_END,
];

type ScriptEol = '\r\n' | '\n' | '\r';

export interface PreparedCampaignCodes {
  codes: string[];
  invalidCount: number;
}

export type CampaignGuardState =
  | { status: 'none' }
  | { status: 'valid'; content: string; codes: string[]; eol: ScriptEol }
  | { status: 'malformed'; error: string };

export type CampaignGuardMutationResult =
  | { ok: true; content: string; codes: string[] }
  | { ok: false; error: string };

const countOccurrences = (value: string, search: string) => {
  let count = 0;
  let offset = 0;
  while (offset < value.length) {
    const index = value.indexOf(search, offset);
    if (index === -1) break;
    count++;
    offset = index + search.length;
  }
  return count;
};

const getEolAt = (value: string, offset: number): ScriptEol | null => {
  if (value.startsWith('\r\n', offset)) return '\r\n';
  if (value.startsWith('\n', offset)) return '\n';
  if (value.startsWith('\r', offset)) return '\r';
  return null;
};

const detectEol = (value: string): ScriptEol => {
  const match = value.match(/\r\n|\n|\r/);
  return (match?.[0] as ScriptEol | undefined) || '\n';
};

const hasTrailingEol = (value: string) => /(?:\r\n|\n|\r)$/.test(value);

export const prepareCampaignCodes = (values: unknown[]): PreparedCampaignCodes => {
  const codes: string[] = [];
  const seen = new Set<string>();
  let invalidCount = 0;

  for (const value of values) {
    if (typeof value !== 'string') {
      invalidCount++;
      continue;
    }

    const code = value.trim();
    if (!code || !MAP_CODE_PATTERN.test(code)) {
      invalidCount++;
      continue;
    }

    const normalized = code.toLowerCase();
    if (seen.has(normalized)) continue;
    seen.add(normalized);
    codes.push(normalized);
  }

  return { codes, invalidCount };
};

const buildGuardPrefix = (codes: string[], eol: ScriptEol) => {
  const conditions = codes
    .map(
      (code, index) =>
        `    Director.GetMapName().tolower() == ${JSON.stringify(code)}${
          index < codes.length - 1 ? ' ||' : ''
        }`
    )
    .join(eol);

  return [
    GUARD_BEGIN,
    `${CODES_PREFIX}${JSON.stringify(codes)}`,
    'if (',
    conditions,
    ')',
    '{',
    CONTENT_BEGIN,
    '',
  ].join(eol);
};

const buildGuardSuffix = (eol: ScriptEol) =>
  [
    '',
    CONTENT_END,
    '}',
    GUARD_END,
  ].join(eol);

export const inspectCampaignGuard = (value: string): CampaignGuardState => {
  const hasManagedMarker = MANAGED_MARKERS.some((marker) => value.includes(marker));
  if (!hasManagedMarker) return { status: 'none' };

  if (MANAGED_MARKERS.some((marker) => countOccurrences(value, marker) !== 1)) {
    return {
      status: 'malformed',
      error: '检测到重复或残缺的战役限定标记，请先手动检查脚本。',
    };
  }
  if (!value.startsWith(GUARD_BEGIN)) {
    return {
      status: 'malformed',
      error: '战役限定标记不在脚本最外层，无法安全自动修改。',
    };
  }

  const eol = getEolAt(value, GUARD_BEGIN.length);
  if (!eol) {
    return {
      status: 'malformed',
      error: '战役限定开始标记格式不完整，无法安全自动修改。',
    };
  }

  const metadataStart = GUARD_BEGIN.length + eol.length;
  const metadataEnd = value.indexOf(eol, metadataStart);
  if (metadataEnd === -1) {
    return {
      status: 'malformed',
      error: '战役章节信息不完整，无法安全自动修改。',
    };
  }
  const metadataLine = value.slice(metadataStart, metadataEnd);
  if (!metadataLine.startsWith(CODES_PREFIX)) {
    return {
      status: 'malformed',
      error: '战役章节标记格式不正确，无法安全自动修改。',
    };
  }

  let parsedCodes: unknown;
  try {
    parsedCodes = JSON.parse(metadataLine.slice(CODES_PREFIX.length));
  } catch {
    return {
      status: 'malformed',
      error: '战役章节标记无法解析，无法安全自动修改。',
    };
  }
  if (!Array.isArray(parsedCodes) || parsedCodes.length === 0) {
    return {
      status: 'malformed',
      error: '战役限定中没有有效的章节 Code。',
    };
  }

  const prepared = prepareCampaignCodes(parsedCodes);
  if (
    prepared.invalidCount > 0 ||
    prepared.codes.length !== parsedCodes.length ||
    parsedCodes.some((code, index) => code !== prepared.codes[index])
  ) {
    return {
      status: 'malformed',
      error: '战役限定中的章节 Code 格式不正确或存在重复。',
    };
  }

  const prefix = buildGuardPrefix(prepared.codes, eol);
  const suffix = buildGuardSuffix(eol);
  const body = value.endsWith(eol) ? value.slice(0, -eol.length) : value;
  if (!body.startsWith(prefix) || !body.endsWith(suffix)) {
    return {
      status: 'malformed',
      error: '战役限定包装已被修改或结构不完整，无法安全自动处理。',
    };
  }

  return {
    status: 'valid',
    content: body.slice(prefix.length, body.length - suffix.length),
    codes: prepared.codes,
    eol,
  };
};

export const applyCampaignGuard = (
  value: string,
  requestedCodes: unknown[]
): CampaignGuardMutationResult => {
  const state = inspectCampaignGuard(value);
  if (state.status === 'malformed') return { ok: false, error: state.error };

  const prepared = prepareCampaignCodes(requestedCodes);
  if (prepared.codes.length === 0) {
    return { ok: false, error: '所选战役没有可用的章节 Code。' };
  }

  const source = state.status === 'valid' ? state.content : value;
  const eol = state.status === 'valid' ? state.eol : detectEol(source);
  const content = `${buildGuardPrefix(prepared.codes, eol)}${source}${buildGuardSuffix(eol)}${
    hasTrailingEol(source) ? eol : ''
  }`;

  return { ok: true, content, codes: prepared.codes };
};

export const removeCampaignGuard = (value: string): CampaignGuardMutationResult => {
  const state = inspectCampaignGuard(value);
  if (state.status === 'malformed') return { ok: false, error: state.error };
  if (state.status === 'none') {
    return { ok: false, error: '当前脚本没有可移除的战役限定。' };
  }
  return { ok: true, content: state.content, codes: [] };
};
