/**
 * Audio Service
 *
 * Synthesizes and plays notification sounds using the Web Audio API.
 * Each sound is generated from oscillator patterns — no external files needed.
 * Sounds are lazily created and cached as AudioBuffers for instant playback.
 */

export type SoundName =
  | 'none'
  | 'chaching'
  | 'synth'
  | 'jazz'
  | 'chime'
  | 'ding'
  | 'droplet'
  | 'pulse'
  | 'bell'
  | 'pop'
  | 'whoosh'
  | 'tap';

export const SOUND_LABELS: Record<SoundName, string> = {
  none: 'None',
  chaching: 'Cha-Ching',
  synth: 'Synth',
  jazz: 'Jazz',
  chime: 'Chime',
  ding: 'Ding',
  droplet: 'Droplet',
  pulse: 'Pulse',
  bell: 'Bell',
  pop: 'Pop',
  whoosh: 'Whoosh',
  tap: 'Tap',
};

export const ALL_SOUNDS: SoundName[] = [
  'none',
  'chaching',
  'synth',
  'jazz',
  'chime',
  'ding',
  'droplet',
  'pulse',
  'bell',
  'pop',
  'whoosh',
  'tap',
];

type SoundGenerator = (ctx: OfflineAudioContext) => void;

const SAMPLE_RATE = 44100;

/** Render an oscillator-based sound into an AudioBuffer. */
function renderSound(generator: SoundGenerator, durationSec: number): Promise<AudioBuffer> {
  const length = Math.ceil(SAMPLE_RATE * durationSec);
  const offline = new OfflineAudioContext(1, length, SAMPLE_RATE);
  generator(offline);
  return offline.startRendering();
}

// ---------------------------------------------------------------------------
// Sound generators — each creates a distinct notification tone
// ---------------------------------------------------------------------------

function genChime(ctx: OfflineAudioContext) {
  // Two-tone ascending chime
  const g = ctx.createGain();
  g.gain.setValueAtTime(0.4, 0);
  g.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.9);
  g.connect(ctx.destination);

  const o1 = ctx.createOscillator();
  o1.type = 'sine';
  o1.frequency.setValueAtTime(880, 0);
  o1.connect(g);
  o1.start(0);
  o1.stop(0.45);

  const g2 = ctx.createGain();
  g2.gain.setValueAtTime(0.4, 0.2);
  g2.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.9);
  g2.connect(ctx.destination);

  const o2 = ctx.createOscillator();
  o2.type = 'sine';
  o2.frequency.setValueAtTime(1318.5, 0);
  o2.connect(g2);
  o2.start(0.2);
  o2.stop(0.9);
}

function genDing(ctx: OfflineAudioContext) {
  // Single bright ding with harmonic overtone
  const g = ctx.createGain();
  g.gain.setValueAtTime(0.5, 0);
  g.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.5);
  g.connect(ctx.destination);

  const o = ctx.createOscillator();
  o.type = 'sine';
  o.frequency.setValueAtTime(1046.5, 0);
  o.connect(g);
  o.start(0);
  o.stop(0.5);

  const gH = ctx.createGain();
  gH.gain.setValueAtTime(0.15, 0);
  gH.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.3);
  gH.connect(ctx.destination);

  const oH = ctx.createOscillator();
  oH.type = 'sine';
  oH.frequency.setValueAtTime(2093, 0);
  oH.connect(gH);
  oH.start(0);
  oH.stop(0.3);
}

function genDroplet(ctx: OfflineAudioContext) {
  // Water droplet: quick descending sine with resonance
  const g = ctx.createGain();
  g.gain.setValueAtTime(0.5, 0);
  g.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.6);
  g.connect(ctx.destination);

  const o = ctx.createOscillator();
  o.type = 'sine';
  o.frequency.setValueAtTime(1600, 0);
  o.frequency.exponentialRampToValueAtTime(400, ctx.currentTime + 0.15);
  o.connect(g);
  o.start(0);
  o.stop(0.6);
}

function genPulse(ctx: OfflineAudioContext) {
  // Soft electronic pulse: triangle wave with vibrato
  const g = ctx.createGain();
  g.gain.setValueAtTime(0, 0);
  g.gain.linearRampToValueAtTime(0.4, 0.05);
  g.gain.linearRampToValueAtTime(0.4, 0.35);
  g.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.7);
  g.connect(ctx.destination);

  const o = ctx.createOscillator();
  o.type = 'triangle';
  o.frequency.setValueAtTime(660, 0);
  o.connect(g);
  o.start(0);
  o.stop(0.7);

  // Subtle vibrato
  const lfo = ctx.createOscillator();
  lfo.type = 'sine';
  lfo.frequency.setValueAtTime(6, 0);
  const lfoGain = ctx.createGain();
  lfoGain.gain.setValueAtTime(8, 0);
  lfo.connect(lfoGain);
  lfoGain.connect(o.frequency);
  lfo.start(0);
  lfo.stop(0.7);
}

function genBell(ctx: OfflineAudioContext) {
  // Small bell: fundamental + inharmonic partials
  const freqs = [523.25, 1568, 2349, 3136];
  const amps = [0.35, 0.15, 0.08, 0.04];
  const decays = [1.0, 0.6, 0.4, 0.3];

  for (let i = 0; i < freqs.length; i++) {
    const g = ctx.createGain();
    g.gain.setValueAtTime(amps[i], 0);
    g.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + decays[i]);
    g.connect(ctx.destination);

    const o = ctx.createOscillator();
    o.type = 'sine';
    o.frequency.setValueAtTime(freqs[i], 0);
    o.connect(g);
    o.start(0);
    o.stop(decays[i]);
  }
}

function genPop(ctx: OfflineAudioContext) {
  // Quick pop/bubble: short sine burst with pitch drop
  const g = ctx.createGain();
  g.gain.setValueAtTime(0.6, 0);
  g.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.15);
  g.connect(ctx.destination);

  const o = ctx.createOscillator();
  o.type = 'sine';
  o.frequency.setValueAtTime(800, 0);
  o.frequency.exponentialRampToValueAtTime(200, ctx.currentTime + 0.1);
  o.connect(g);
  o.start(0);
  o.stop(0.15);
}

function genWhoosh(ctx: OfflineAudioContext) {
  // Gentle whoosh: filtered noise sweep
  const length = Math.ceil(SAMPLE_RATE * 0.6);
  const noise = ctx.createBufferSource();
  const noiseBuffer = ctx.createBuffer(1, length, SAMPLE_RATE);
  const data = noiseBuffer.getChannelData(0);
  for (let i = 0; i < length; i++) {
    data[i] = Math.random() * 2 - 1;
  }
  noise.buffer = noiseBuffer;

  const filter = ctx.createBiquadFilter();
  filter.type = 'bandpass';
  filter.frequency.setValueAtTime(400, 0);
  filter.frequency.exponentialRampToValueAtTime(4000, 0.3);
  filter.frequency.exponentialRampToValueAtTime(400, 0.6);
  filter.Q.setValueAtTime(2, 0);

  const g = ctx.createGain();
  g.gain.setValueAtTime(0, 0);
  g.gain.linearRampToValueAtTime(0.3, 0.15);
  g.gain.linearRampToValueAtTime(0.3, 0.35);
  g.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.6);

  noise.connect(filter);
  filter.connect(g);
  g.connect(ctx.destination);
  noise.start(0);
}

function genTap(ctx: OfflineAudioContext) {
  // Crisp tap: very short noise burst + high sine
  const length = Math.ceil(SAMPLE_RATE * 0.15);
  const noise = ctx.createBufferSource();
  const noiseBuffer = ctx.createBuffer(1, length, SAMPLE_RATE);
  const data = noiseBuffer.getChannelData(0);
  for (let i = 0; i < length; i++) {
    data[i] = Math.random() * 2 - 1;
  }
  noise.buffer = noiseBuffer;

  const filter = ctx.createBiquadFilter();
  filter.type = 'highpass';
  filter.frequency.setValueAtTime(4000, 0);

  const g = ctx.createGain();
  g.gain.setValueAtTime(0.5, 0);
  g.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.08);

  noise.connect(filter);
  filter.connect(g);
  g.connect(ctx.destination);
  noise.start(0);

  // Add a click tone
  const gT = ctx.createGain();
  gT.gain.setValueAtTime(0.3, 0);
  gT.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.05);
  gT.connect(ctx.destination);

  const oT = ctx.createOscillator();
  oT.type = 'sine';
  oT.frequency.setValueAtTime(3000, 0);
  oT.connect(gT);
  oT.start(0);
  oT.stop(0.05);
}

function genChaChing(ctx: OfflineAudioContext) {
  // Cash register cha-ching: two bright metallic hits + shimmer
  const hit1 = ctx.createOscillator();
  hit1.type = 'square';
  hit1.frequency.setValueAtTime(1800, 0);
  hit1.frequency.exponentialRampToValueAtTime(900, 0.06);
  const g1 = ctx.createGain();
  g1.gain.setValueAtTime(0.4, 0);
  g1.gain.exponentialRampToValueAtTime(0.001, 0.12);
  hit1.connect(g1);
  g1.connect(ctx.destination);
  hit1.start(0);
  hit1.stop(0.12);

  // Second hit (higher, delayed)
  const hit2 = ctx.createOscillator();
  hit2.type = 'square';
  hit2.frequency.setValueAtTime(2400, 0.12);
  hit2.frequency.exponentialRampToValueAtTime(1200, 0.18);
  const g2 = ctx.createGain();
  g2.gain.setValueAtTime(0.5, 0.12);
  g2.gain.exponentialRampToValueAtTime(0.001, 0.35);
  hit2.connect(g2);
  g2.connect(ctx.destination);
  hit2.start(0.12);
  hit2.stop(0.35);

  // Shimmer tail: high sine ring-out
  const shimmer = ctx.createOscillator();
  shimmer.type = 'sine';
  shimmer.frequency.setValueAtTime(4186, 0.15);
  const gs = ctx.createGain();
  gs.gain.setValueAtTime(0.2, 0.15);
  gs.gain.exponentialRampToValueAtTime(0.001, 0.7);
  shimmer.connect(gs);
  gs.connect(ctx.destination);
  shimmer.start(0.15);
  shimmer.stop(0.7);
}

function genSynth(ctx: OfflineAudioContext) {
  // AI/futuristic synth: saw wave with filter sweep + sub bass
  const saw = ctx.createOscillator();
  saw.type = 'sawtooth';
  saw.frequency.setValueAtTime(220, 0);
  saw.frequency.linearRampToValueAtTime(440, 0.3);
  saw.frequency.linearRampToValueAtTime(330, 0.6);

  const filter = ctx.createBiquadFilter();
  filter.type = 'lowpass';
  filter.frequency.setValueAtTime(200, 0);
  filter.frequency.exponentialRampToValueAtTime(6000, 0.25);
  filter.frequency.exponentialRampToValueAtTime(800, 0.8);
  filter.Q.setValueAtTime(8, 0);

  const g = ctx.createGain();
  g.gain.setValueAtTime(0, 0);
  g.gain.linearRampToValueAtTime(0.3, 0.05);
  g.gain.linearRampToValueAtTime(0.25, 0.4);
  g.gain.exponentialRampToValueAtTime(0.001, 0.9);

  saw.connect(filter);
  filter.connect(g);
  g.connect(ctx.destination);
  saw.start(0);
  saw.stop(0.9);

  // Sub bass layer
  const sub = ctx.createOscillator();
  sub.type = 'sine';
  sub.frequency.setValueAtTime(110, 0);
  const gSub = ctx.createGain();
  gSub.gain.setValueAtTime(0, 0);
  gSub.gain.linearRampToValueAtTime(0.2, 0.05);
  gSub.gain.exponentialRampToValueAtTime(0.001, 0.6);
  sub.connect(gSub);
  gSub.connect(ctx.destination);
  sub.start(0);
  sub.stop(0.6);
}

function genJazz(ctx: OfflineAudioContext) {
  // Jazzy: muted trumpet-like tone with vibrato + walking bass note
  // Trumpet: triangle wave with gentle vibrato
  const trumpet = ctx.createOscillator();
  trumpet.type = 'triangle';
  trumpet.frequency.setValueAtTime(587.33, 0); // D5

  const vibLfo = ctx.createOscillator();
  vibLfo.type = 'sine';
  vibLfo.frequency.setValueAtTime(5.5, 0);
  const vibDepth = ctx.createGain();
  vibDepth.gain.setValueAtTime(12, 0);
  vibLfo.connect(vibDepth);
  vibDepth.connect(trumpet.frequency);
  vibLfo.start(0);
  vibLfo.stop(0.8);

  const mute = ctx.createBiquadFilter();
  mute.type = 'lowpass';
  mute.frequency.setValueAtTime(1200, 0);
  mute.Q.setValueAtTime(3, 0);

  const gT = ctx.createGain();
  gT.gain.setValueAtTime(0, 0);
  gT.gain.linearRampToValueAtTime(0.35, 0.04);
  gT.gain.linearRampToValueAtTime(0.3, 0.3);
  gT.gain.exponentialRampToValueAtTime(0.001, 0.8);

  trumpet.connect(mute);
  mute.connect(gT);
  gT.connect(ctx.destination);
  trumpet.start(0);
  trumpet.stop(0.8);

  // Walking bass pluck
  const bass = ctx.createOscillator();
  bass.type = 'sine';
  bass.frequency.setValueAtTime(146.83, 0.05); // D3
  const gB = ctx.createGain();
  gB.gain.setValueAtTime(0.3, 0.05);
  gB.gain.exponentialRampToValueAtTime(0.001, 0.45);
  bass.connect(gB);
  gB.connect(ctx.destination);
  bass.start(0.05);
  bass.stop(0.45);

  // Second bass note
  const bass2 = ctx.createOscillator();
  bass2.type = 'sine';
  bass2.frequency.setValueAtTime(196, 0.35); // G3
  const gB2 = ctx.createGain();
  gB2.gain.setValueAtTime(0.25, 0.35);
  gB2.gain.exponentialRampToValueAtTime(0.001, 0.7);
  bass2.connect(gB2);
  gB2.connect(ctx.destination);
  bass2.start(0.35);
  bass2.stop(0.7);
}

const SOUND_GENERATORS: Record<SoundName, { gen: SoundGenerator; duration: number }> = {
  none:     { gen: () => {},      duration: 0   },
  chaching: { gen: genChaChing,   duration: 0.8 },
  synth:    { gen: genSynth,      duration: 1.0 },
  jazz:     { gen: genJazz,       duration: 0.9 },
  chime:    { gen: genChime,      duration: 1.0 },
  ding:     { gen: genDing,       duration: 0.5 },
  droplet:  { gen: genDroplet,    duration: 0.7 },
  pulse:    { gen: genPulse,      duration: 0.8 },
  bell:     { gen: genBell,       duration: 1.0 },
  pop:      { gen: genPop,        duration: 0.3 },
  whoosh:   { gen: genWhoosh,     duration: 0.7 },
  tap:      { gen: genTap,        duration: 0.2 },
};

// ---------------------------------------------------------------------------
// AudioService singleton
// ---------------------------------------------------------------------------

class AudioService {
  private cache = new Map<SoundName, AudioBuffer>();
  private ctx: AudioContext | null = null;

  private getContext(): AudioContext {
    if (!this.ctx) {
      this.ctx = new AudioContext();
    }
    return this.ctx;
  }

  /** Resume AudioContext (must be called from a user gesture on first use). */
  async resume(): Promise<void> {
    const ctx = this.getContext();
    if (ctx.state === 'suspended') {
      await ctx.resume();
    }
  }

  /** Pre-render a sound into the cache. */
  async preload(name: SoundName): Promise<void> {
    if (name === 'none' || this.cache.has(name)) return;
    const spec = SOUND_GENERATORS[name];
    const buffer = await renderSound(spec.gen, spec.duration);
    this.cache.set(name, buffer);
  }

  /** Pre-render all sounds. */
  async preloadAll(): Promise<void> {
    await Promise.all(ALL_SOUNDS.map((n) => this.preload(n)));
  }

  /** Play a notification sound at the given volume (0–1). */
  async play(name: SoundName, volume = 0.7): Promise<void> {
    if (name === 'none') return;
    await this.resume();
    await this.preload(name);

    const buffer = this.cache.get(name);
    if (!buffer) return;

    const ctx = this.getContext();
    const source = ctx.createBufferSource();
    source.buffer = buffer;

    const gain = ctx.createGain();
    gain.gain.setValueAtTime(Math.max(0, Math.min(1, volume)), ctx.currentTime);

    source.connect(gain);
    gain.connect(ctx.destination);
    source.start(0);
  }
}

export const audioService = new AudioService();
