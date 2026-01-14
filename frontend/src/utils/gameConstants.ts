export const DIFFICULTY_MAP: Record<string, string> = {
  Easy: '简单',
  Normal: '普通',
  Hard: '高级',
  Impossible: '专家',
};

export const GAMEMODE_MAP: Record<string, string> = {
  coop: '合作',
  versus: '对抗',
  survival: '生存',
  realism: '写实',
  scavenge: '清道夫',
  realismversus: '写实对抗',
  mutation: '突变',
  // Special case: server sends 拾荒, display as 清道夫
  拾荒: '清道夫',

  // Mutation Modes Mappings (English Code -> Chinese Display)
  lastmanonearth: '地球上最后一人',
  headshot: '爆头！',
  bleedout: '血流不止',
  hardeight: '绝境求生',
  fourswordsmen: '四剑客',
  chainsawmassacre: '链锯屠杀',
  ironman: '铁人',
  lastgnomeonearth: '地球上最后侏儒',
  roomforone: '仅容一人',
  healthpackalypse: '医疗末日',
  followtheliter: '跟随公升',
  gibfest: '碎尸盛宴',
  versussurvival: '对抗生存',
  huntingparty: '猎杀派对',
  lonegunman: '孤胆枪手',
  bleedoutversus: '失血对抗',
  taaannnkk: '无尽坦克！',
  healinggnome: '治疗侏儒',

  // Community Modes Mappings
  specialdelivery: '特感速递',
  fluseason: '流感季节',
  ridingmysurvivor: '骑乘派对',
  nightmare: '梦魇',
  deathsdoor: '死亡之门',
  confogl: 'Confogl',
};

export const DIFFICULTY_OPTIONS = [
  { value: '简单', label: '简单', desc: 'Easy', icon: '🟢', color: 'green' },
  { value: '普通', label: '普通', desc: 'Normal', icon: '🟡', color: 'yellow' },
  { value: '高级', label: '高级', desc: 'Hard', icon: '🟠', color: 'orange' },
  { value: '专家', label: '专家', desc: 'Impossible', icon: '🔴', color: 'red' },
];

export const GAMEMODE_OPTIONS = [
  // Base Modes
  { value: '合作', label: '合作', desc: 'Cooperative', icon: '🏃', color: 'blue', type: 'base' },
  { value: '写实', label: '写实', desc: 'Realism', icon: '💀', color: 'purple', type: 'base' },
  { value: '生存', label: '生存', desc: 'Survival', icon: '🛡️', color: 'orange', type: 'base' },
  { value: '对抗', label: '对抗', desc: 'Versus', icon: '⚔️', color: 'red', type: 'base' },
  { value: '拾荒', label: '清道夫', desc: 'Scavenge', icon: '⛽', color: 'green', type: 'base' },
  { value: '坚守', label: '坚守', desc: 'Holdout', icon: '🏰', color: 'indigo', type: 'base' },

  // Mutation Modes
  {
    value: '地球上最后一人',
    label: '地球上最后一人',
    desc: 'Last Man on Earth',
    type: 'mutation',
    color: 'pink',
  },
  { value: '爆头！', label: '爆头！', desc: 'Headshot!', type: 'mutation', color: 'pink' },
  { value: '血流不止', label: '血流不止', desc: 'Bleed Out', type: 'mutation', color: 'pink' },
  { value: '绝境求生', label: '绝境求生', desc: 'Hard Eight', type: 'mutation', color: 'pink' },
  { value: '四剑客', label: '四剑客', desc: 'Four Swordsmen', type: 'mutation', color: 'pink' },
  {
    value: '链锯屠杀',
    label: '链锯屠杀',
    desc: 'Chainsaw Massacre',
    type: 'mutation',
    color: 'pink',
  },
  { value: '铁人', label: '铁人', desc: 'Iron Man', type: 'mutation', color: 'pink' },
  {
    value: '地球上最后侏儒',
    label: '地球上最后侏儒',
    desc: 'Last Gnome on Earth',
    type: 'mutation',
    color: 'pink',
  },
  { value: '仅容一人', label: '仅容一人', desc: 'Room for One', type: 'mutation', color: 'pink' },
  {
    value: '医疗末日',
    label: '医疗末日',
    desc: 'Healthpackalypse!',
    type: 'mutation',
    color: 'pink',
  },
  { value: '写实对抗', label: '写实对抗', desc: 'Realism Versus', type: 'mutation', color: 'pink' },
  {
    value: '跟随公升',
    label: '跟随公升',
    desc: 'Follow the Liter',
    type: 'mutation',
    color: 'pink',
  },
  { value: '碎尸盛宴', label: '碎尸盛宴', desc: 'Gib Fest', type: 'mutation', color: 'pink' },
  {
    value: '对抗生存',
    label: '对抗生存',
    desc: 'Versus Survival',
    type: 'mutation',
    color: 'pink',
  },
  { value: '猎杀派对', label: '猎杀派对', desc: 'Hunting Party', type: 'mutation', color: 'pink' },
  { value: '孤胆枪手', label: '孤胆枪手', desc: 'Lone Gunman', type: 'mutation', color: 'pink' },
  {
    value: '失血对抗',
    label: '失血对抗',
    desc: 'Bleed Out Versus',
    type: 'mutation',
    color: 'pink',
  },
  { value: '无尽坦克！', label: '无尽坦克！', desc: 'Taaannnkk!', type: 'mutation', color: 'pink' },
  { value: '治疗侏儒', label: '治疗侏儒', desc: 'Healing Gnome', type: 'mutation', color: 'pink' },

  // Community Modes
  {
    value: '特感速递',
    label: '特感速递',
    desc: 'Special Delivery',
    type: 'community',
    color: 'teal',
  },
  { value: '流感季节', label: '流感季节', desc: 'Flu Season', type: 'community', color: 'teal' },
  {
    value: '骑乘派对',
    label: '骑乘派对',
    desc: 'Riding My Survivor',
    type: 'community',
    color: 'teal',
  },
  { value: '梦魇', label: '梦魇', desc: 'Nightmare', type: 'community', color: 'teal' },
  { value: '死亡之门', label: '死亡之门', desc: "Death's Door", type: 'community', color: 'teal' },
  { value: 'Confogl', label: 'Confogl', desc: 'Confogl', type: 'community', color: 'teal' },
];
