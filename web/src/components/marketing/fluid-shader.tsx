"use client";

/*
 * FluidShader — vendored from Framer's FluidShaderBackground marketplace
 * component (https://framer.com/m/FluidShaderBackground-unmWDO.js, free
 * tier). Stripped of the in-canvas color-picker UI and Framer-only imports
 * (addPropertyControls, ControlType, useIsStaticRenderer); theme is now a
 * prop. Pauses animation when the canvas is offscreen.
 */

import { useEffect, useRef } from "react";

// Theme palettes, kept in [main, low, mid, high] format used by the shader.
// The original Framer palettes (orange/blue/purple/...) are preserved as-is
// for compatibility, plus a `dark*` family tuned for AgentClash: deeper
// base, single accent hue, designed to read as a dark tinted card rather
// than a vibrant gradient.
const THEMES = {
  orange: { main: [1, 0.95, 0.7], low: [0.95, 0.75, 0.4], mid: [0.98, 0.7, 0.6], high: [1, 1, 1] },
  blue: { main: [0.7, 0.85, 1], low: [0.4, 0.6, 0.9], mid: [0.5, 0.7, 1], high: [0.9, 0.95, 1] },
  purple: { main: [0.9, 0.75, 1], low: [0.6, 0.45, 0.9], mid: [0.7, 0.55, 1], high: [0.95, 0.9, 1] },
  green: { main: [0.75, 1, 0.85], low: [0.4, 0.8, 0.6], mid: [0.5, 0.9, 0.7], high: [0.9, 1, 0.95] },
  crimson: { main: [1, 0.75, 0.75], low: [0.9, 0.5, 0.5], mid: [1, 0.6, 0.6], high: [1, 0.9, 0.9] },
  teal: { main: [0.7, 1, 0.95], low: [0.4, 0.85, 0.8], mid: [0.5, 0.95, 0.9], high: [0.9, 1, 0.98] },
  pink: { main: [1, 0.8, 0.95], low: [0.95, 0.5, 0.8], mid: [1, 0.65, 0.9], high: [1, 0.95, 0.98] },
  yellow: { main: [1, 0.98, 0.7], low: [0.95, 0.85, 0.4], mid: [1, 0.92, 0.55], high: [1, 1, 0.95] },
  indigo: { main: [0.75, 0.8, 1], low: [0.45, 0.5, 0.9], mid: [0.6, 0.65, 1], high: [0.9, 0.92, 1] },
  mint: { main: [0.8, 1, 0.9], low: [0.5, 0.9, 0.7], mid: [0.65, 1, 0.8], high: [0.95, 1, 0.97] },
  coral: { main: [1, 0.85, 0.8], low: [0.95, 0.6, 0.5], mid: [1, 0.72, 0.65], high: [1, 0.95, 0.93] },
  lavender: { main: [0.92, 0.85, 1], low: [0.7, 0.6, 0.95], mid: [0.82, 0.72, 1], high: [0.98, 0.95, 1] },
  peach: { main: [1, 0.9, 0.8], low: [0.95, 0.7, 0.55], mid: [1, 0.8, 0.68], high: [1, 0.97, 0.93] },
  sky: { main: [0.8, 0.95, 1], low: [0.5, 0.8, 0.95], mid: [0.65, 0.88, 1], high: [0.95, 0.98, 1] },
  rose: { main: [1, 0.75, 0.85], low: [0.95, 0.45, 0.65], mid: [1, 0.6, 0.75], high: [1, 0.92, 0.95] },
  // AgentClash-tuned dark palettes — main lives near pure black so the
  // card "blends in", with a single hue carried in mid/high for the
  // hover-reveal lift.
  darkSlate:   { main: [0.05, 0.06, 0.09], low: [0.10, 0.13, 0.20], mid: [0.18, 0.26, 0.42], high: [0.55, 0.70, 0.95] },
  darkAmber:   { main: [0.07, 0.05, 0.03], low: [0.16, 0.10, 0.05], mid: [0.36, 0.22, 0.10], high: [0.85, 0.62, 0.30] },
  darkCrimson: { main: [0.07, 0.04, 0.05], low: [0.18, 0.07, 0.10], mid: [0.40, 0.13, 0.20], high: [0.85, 0.40, 0.50] },
  darkViolet:  { main: [0.06, 0.05, 0.09], low: [0.13, 0.10, 0.22], mid: [0.28, 0.20, 0.42], high: [0.65, 0.50, 0.90] },
  darkEmerald: { main: [0.04, 0.07, 0.06], low: [0.07, 0.16, 0.12], mid: [0.13, 0.32, 0.24], high: [0.40, 0.78, 0.60] },
} as const;

export type FluidShaderTheme = keyof typeof THEMES;

const VS = `#version 300 es
in vec2 a_position;
out vec2 out_uv;
void main() {
  out_uv = a_position * 0.5 + 0.5;
  out_uv.y = 1.0 - out_uv.y;
  gl_Position = vec4(a_position, 0.0, 1.0);
}`;

const FS = `#version 300 es
precision highp float;
// Perf: original Framer shader uses 4 FBM octaves. Three is visually
// indistinguishable for this fluid look and ~25% cheaper per pixel.
#define NUM_OCTAVES 3
in vec2 out_uv;
out vec4 fragColor;
uniform float u_time;
uniform vec2  u_viewport;
uniform sampler2D uTextureNoise;
uniform vec3 u_bloopColorMain;
uniform vec3 u_bloopColorLow;
uniform vec3 u_bloopColorMid;
uniform vec3 u_bloopColorHigh;
uniform float u_windSpeed;
uniform float u_warpPower;
uniform float u_fbmStrength;
uniform float u_blurRadius;
uniform float u_zoom;
uniform float u_grainStrength;
uniform float u_grainScale;
uniform float u_noiseScale;

vec3 blendLinearBurn(vec3 base, vec3 blend, float opacity) {
  return max(base + blend - vec3(1.0), vec3(0.0)) * opacity + base * (1.0 - opacity);
}
vec4 permute(vec4 x) { return mod((x * 34.0 + 1.0) * x, 289.0); }
vec4 taylorInvSqrt(vec4 r) { return 1.79284291400159 - 0.85373472095314 * r; }
vec3 fade3(vec3 t) { return t * t * t * (t * (t * 6.0 - 15.0) + 10.0); }
float rand(vec2 n) { return fract(sin(dot(n, vec2(12.9898, 4.1414))) * 43758.5453); }
float noise2(vec2 p) {
  vec2 ip = floor(p); vec2 u = fract(p);
  u = u * u * (3.0 - 2.0 * u);
  return pow(mix(mix(rand(ip), rand(ip+vec2(1,0)), u.x), mix(rand(ip+vec2(0,1)), rand(ip+vec2(1,1)), u.x), u.y), 2.0);
}
float fbm(vec2 x) {
  float v = 0.0; float a = 0.5;
  vec2 shift = vec2(100.0);
  mat2 rot = mat2(cos(0.5), sin(0.5), -sin(0.5), cos(0.5));
  for (int i = 0; i < NUM_OCTAVES; ++i) { v += a * noise2(x); x = rot * x * 2.0 + shift; a *= 0.5; }
  return v;
}
float cnoise(vec3 P) {
  vec3 Pi0 = floor(P); vec3 Pi1 = Pi0 + vec3(1.0);
  Pi0 = mod(Pi0, 289.0); Pi1 = mod(Pi1, 289.0);
  vec3 Pf0 = fract(P); vec3 Pf1 = Pf0 - vec3(1.0);
  vec4 ix = vec4(Pi0.x,Pi1.x,Pi0.x,Pi1.x);
  vec4 iy = vec4(Pi0.yy,Pi1.yy);
  vec4 iz0 = vec4(Pi0.z); vec4 iz1 = vec4(Pi1.z);
  vec4 ixy = permute(permute(ix)+iy);
  vec4 ixy0 = permute(ixy+iz0); vec4 ixy1 = permute(ixy+iz1);
  vec4 gx0=ixy0/7.0; vec4 gy0=fract(floor(gx0)/7.0)-0.5; gx0=fract(gx0);
  vec4 gz0=vec4(0.5)-abs(gx0)-abs(gy0); vec4 sz0=step(gz0,vec4(0.0));
  gx0-=sz0*(step(vec4(0.0),gx0)-0.5); gy0-=sz0*(step(vec4(0.0),gy0)-0.5);
  vec4 gx1=ixy1/7.0; vec4 gy1=fract(floor(gx1)/7.0)-0.5; gx1=fract(gx1);
  vec4 gz1=vec4(0.5)-abs(gx1)-abs(gy1); vec4 sz1=step(gz1,vec4(0.0));
  gx1-=sz1*(step(vec4(0.0),gx1)-0.5); gy1-=sz1*(step(vec4(0.0),gy1)-0.5);
  vec3 g000=vec3(gx0.x,gy0.x,gz0.x); vec3 g100=vec3(gx0.y,gy0.y,gz0.y);
  vec3 g010=vec3(gx0.z,gy0.z,gz0.z); vec3 g110=vec3(gx0.w,gy0.w,gz0.w);
  vec3 g001=vec3(gx1.x,gy1.x,gz1.x); vec3 g101=vec3(gx1.y,gy1.y,gz1.y);
  vec3 g011=vec3(gx1.z,gy1.z,gz1.z); vec3 g111=vec3(gx1.w,gy1.w,gz1.w);
  vec4 norm0=taylorInvSqrt(vec4(dot(g000,g000),dot(g010,g010),dot(g100,g100),dot(g110,g110)));
  g000*=norm0.x; g010*=norm0.y; g100*=norm0.z; g110*=norm0.w;
  vec4 norm1=taylorInvSqrt(vec4(dot(g001,g001),dot(g011,g011),dot(g101,g101),dot(g111,g111)));
  g001*=norm1.x; g011*=norm1.y; g101*=norm1.z; g111*=norm1.w;
  float n000=dot(g000,Pf0); float n100=dot(g100,vec3(Pf1.x,Pf0.yz));
  float n010=dot(g010,vec3(Pf0.x,Pf1.y,Pf0.z)); float n110=dot(g110,vec3(Pf1.xy,Pf0.z));
  float n001=dot(g001,vec3(Pf0.xy,Pf1.z)); float n101=dot(g101,vec3(Pf1.x,Pf0.y,Pf1.z));
  float n011=dot(g011,vec3(Pf0.x,Pf1.yz)); float n111=dot(g111,Pf1);
  vec3 fxyz=fade3(Pf0);
  vec4 nz=mix(vec4(n000,n100,n010,n110),vec4(n001,n101,n011,n111),fxyz.z);
  vec2 nyz=mix(nz.xy,nz.zw,fxyz.y);
  return 2.2*mix(nyz.x,nyz.y,fxyz.x);
}

vec3 getFluidColor(vec2 st, float time) {
  float scaleFactor = 1.0 / (2.0 * u_zoom);
  vec2 uv = st * scaleFactor + 0.5;
  uv.y = 1.0 - uv.y;
  float noiseScale = u_noiseScale;
  float windSpeed  = u_windSpeed;
  float warpPower  = u_warpPower;
  float fbmStrength= u_fbmStrength;
  float blurRadius = u_blurRadius;
  float waterColorNoiseScale    = 18.0;
  float waterColorNoiseStrength = 0.02;
  float textureNoiseScale    = u_grainScale;
  float textureNoiseStrength = u_grainStrength;
  float verticalOffset = 0.09;
  float waveSpread     = 1.0;
  float layer1Amplitude=1.5; float layer1Frequency=1.0;
  float layer2Amplitude=1.4; float layer2Frequency=1.0;
  float layer3Amplitude=1.3; float layer3Frequency=1.0;
  float fbmPowerDamping=0.55;
  time *= 0.85;
  verticalOffset += 1.0 - waveSpread;
  float noiseX = cnoise(vec3(uv*noiseScale+vec2(0.0,74.8572),time*0.3));
  float noiseY = cnoise(vec3(uv*noiseScale+vec2(203.91282,10.0),time*0.3));
  uv += vec2(noiseX*2.0, noiseY) * warpPower;
  float noiseA = cnoise(vec3(uv*waterColorNoiseScale+vec2(344.91282,0.0),time*0.3))
               + cnoise(vec3(uv*waterColorNoiseScale*2.2+vec2(723.937,0.0),time*0.4))*0.5;
  uv += noiseA * waterColorNoiseStrength;
  uv.y -= verticalOffset;
  vec2 texUv = uv * textureNoiseScale;
  float tR0 = texture(uTextureNoise, texUv).r;
  float tG0 = texture(uTextureNoise, vec2(texUv.x,1.0-texUv.y)).g;
  float disp0 = mix(tR0-0.5, tG0-0.5, (sin(time)+1.0)*0.5) * textureNoiseStrength;
  texUv += vec2(63.861,368.937);
  float tR1 = texture(uTextureNoise, texUv).r;
  float tG1 = texture(uTextureNoise, vec2(texUv.x,1.0-texUv.y)).g;
  float disp1 = mix(tR1-0.5, tG1-0.5, (sin(time)+1.0)*0.5) * textureNoiseStrength;
  texUv += vec2(272.861+180.302, 829.937+819.871);
  float tR3 = texture(uTextureNoise, texUv).r;
  float tG3 = texture(uTextureNoise, vec2(texUv.x,1.0-texUv.y)).g;
  float disp3 = mix(tR3-0.5, tG3-0.5, (sin(time)+1.0)*0.5) * textureNoiseStrength;
  uv += disp0;
  vec2 stFbm = uv * noiseScale;
  vec2 q = vec2(fbm(stFbm*0.5+windSpeed*time), fbm(stFbm*0.5+windSpeed*time));
  vec2 r = vec2(
    fbm(stFbm + 1.0*q + vec2(0.3,9.2) + 0.15*time),
    fbm(stFbm + 1.0*q + vec2(8.3,0.8) + 0.126*time)
  );
  float f = fbm(stFbm + r - q);
  float fullFbm = (f + 0.6*f*f + 0.7*f + 0.5) * 0.5;
  fullFbm = pow(fullFbm, fbmPowerDamping) * fbmStrength;
  blurRadius *= 1.5;
  vec2 snUv = (uv + vec2((fullFbm-0.5)*1.2) + vec2(0.0,0.025) + disp0) * vec2(layer1Frequency,1.0);
  float sn  = noise2(snUv*2.0 + vec2(0.0,time*0.5)) * 2.0 * layer1Amplitude;
  float sn2 = pow(smoothstep(sn-1.2*blurRadius, sn+1.2*blurRadius, (snUv.y-0.5*waveSpread)*5.0+0.5), 0.8);
  vec2 snUvB = (uv + vec2((fullFbm-0.5)*0.85) + vec2(0.0,0.025) + disp1) * vec2(layer2Frequency,1.0);
  float snB  = noise2(snUvB*4.0 + vec2(293.0,time*1.0)) * 2.0 * layer2Amplitude;
  float sn2B = pow(smoothstep(snB-0.9*blurRadius, snB+0.9*blurRadius, (snUvB.y-0.6*waveSpread)*5.0+0.5), 0.9);
  vec2 snUvC = (uv + vec2((fullFbm-0.5)*1.1) + disp3) * vec2(layer3Frequency,1.0);
  float snC  = noise2(snUvC*6.0 + vec2(153.0,time*1.2)) * 2.0 * layer3Amplitude;
  float sn2C = smoothstep(snC-0.7*blurRadius, snC+0.7*blurRadius, (snUvC.y-0.9*waveSpread)*6.0+0.5);
  vec3 color = blendLinearBurn(u_bloopColorMain, u_bloopColorLow, 1.0-sn2);
  color = blendLinearBurn(color, mix(u_bloopColorMain, u_bloopColorMid, 1.0-sn2B), sn2);
  color = mix(color, mix(u_bloopColorMain, u_bloopColorHigh, 1.0-sn2C), sn2*sn2B);
  return color;
}

void main() {
  // Patch (vs original Framer shader): the original does
  //   st.x *= viewport.x / viewport.y;
  // which compresses the noise field into a hard horizontal band on the
  // tall, narrow accordion cards. Normalize against the shorter side
  // instead so the wave scale stays consistent regardless of aspect;
  // narrow cards just sample a vertical slice of the same field.
  vec2 st = (out_uv - 0.5) * (u_viewport / min(u_viewport.x, u_viewport.y));
  fragColor = vec4(getFluidColor(st, u_time), 1.0);
}`;

export type FluidShaderProps = {
  theme?: FluidShaderTheme;
  speed?: number;
  windSpeed?: number;
  warpPower?: number;
  fbmStrength?: number;
  blurRadius?: number;
  zoom?: number;
  grainStrength?: number;
  grainScale?: number;
  noiseScale?: number;
  className?: string;
  style?: React.CSSProperties;
};

export function FluidShader({
  theme = "blue",
  speed = 0.72,
  windSpeed = 0.144,
  warpPower = 0.2355,
  fbmStrength = 0.912,
  blurRadius = 1.2673,
  zoom = 0.3971,
  grainStrength = 0.014,
  grainScale = 2.5,
  noiseScale = 0.8673,
  className,
  style,
}: FluidShaderProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const propsRef = useRef({
    theme,
    speed,
    windSpeed,
    warpPower,
    fbmStrength,
    blurRadius,
    zoom,
    grainStrength,
    grainScale,
    noiseScale,
  });
  propsRef.current = {
    theme,
    speed,
    windSpeed,
    warpPower,
    fbmStrength,
    blurRadius,
    zoom,
    grainStrength,
    grainScale,
    noiseScale,
  };

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const gl = canvas.getContext("webgl2");
    if (!gl) return;

    const compile = (type: number, src: string) => {
      const shader = gl.createShader(type)!;
      gl.shaderSource(shader, src);
      gl.compileShader(shader);
      return shader;
    };
    const prog = gl.createProgram()!;
    gl.attachShader(prog, compile(gl.VERTEX_SHADER, VS));
    gl.attachShader(prog, compile(gl.FRAGMENT_SHADER, FS));
    gl.linkProgram(prog);
    if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) return;

    // Random RGB noise texture for grain + warp lookups.
    const size = 256;
    const data = new Uint8Array(size * size * 4);
    for (let i = 0; i < data.length; i += 4) {
      const v = (Math.random() * 255) | 0;
      data[i] = data[i + 1] = data[i + 2] = v;
      data[i + 3] = 255;
    }
    const tex = gl.createTexture();
    gl.bindTexture(gl.TEXTURE_2D, tex);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, size, size, 0, gl.RGBA, gl.UNSIGNED_BYTE, data);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);

    const vao = gl.createVertexArray();
    gl.bindVertexArray(vao);
    const buf = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, buf);
    gl.bufferData(
      gl.ARRAY_BUFFER,
      new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]),
      gl.STATIC_DRAW,
    );
    const loc = gl.getAttribLocation(prog, "a_position");
    gl.enableVertexAttribArray(loc);
    gl.vertexAttribPointer(loc, 2, gl.FLOAT, false, 0, 0);
    gl.bindVertexArray(null);

    const U = {
      time: gl.getUniformLocation(prog, "u_time"),
      viewport: gl.getUniformLocation(prog, "u_viewport"),
      texNoise: gl.getUniformLocation(prog, "uTextureNoise"),
      colMain: gl.getUniformLocation(prog, "u_bloopColorMain"),
      colLow: gl.getUniformLocation(prog, "u_bloopColorLow"),
      colMid: gl.getUniformLocation(prog, "u_bloopColorMid"),
      colHigh: gl.getUniformLocation(prog, "u_bloopColorHigh"),
      windSpeed: gl.getUniformLocation(prog, "u_windSpeed"),
      warpPower: gl.getUniformLocation(prog, "u_warpPower"),
      fbmStr: gl.getUniformLocation(prog, "u_fbmStrength"),
      blurRad: gl.getUniformLocation(prog, "u_blurRadius"),
      zoom: gl.getUniformLocation(prog, "u_zoom"),
      grainStr: gl.getUniformLocation(prog, "u_grainStrength"),
      grainSc: gl.getUniformLocation(prog, "u_grainScale"),
      noiseSc: gl.getUniformLocation(prog, "u_noiseScale"),
    };

    let globalTime = 0;
    let lastTs = performance.now();
    let rafId = 0;
    let isVisible = true;
    // Collapsed accordion cards (the narrow "spines") don't need to animate
    // — the user can't see motion in a 60px-wide strip anyway. We freeze
    // those on the last drawn frame and only animate when expanded.
    let isWide = true;
    // 5 simultaneous WebGL contexts at 60fps was the source of the hangs.
    // 30fps target — half the GPU work, no visible difference for slow
    // fluid motion. `prefers-reduced-motion` users get a single static
    // frame.
    const reducedMotion =
      typeof window !== "undefined" &&
      window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
    const FRAME_BUDGET_MS = 1000 / 30;
    let lastDrawTs = 0;
    let drewOnce = false;

    // Cap DPR aggressively. The shader is a soft fluid background; even at
    // 0.6× the user can't tell, and we save ~3× pixel-shader work vs 1.5×.
    const dpr = Math.min(window.devicePixelRatio || 1, 0.75);
    const COLLAPSED_THRESHOLD_PX = 200;
    const resize = () => {
      const cssW = canvas.clientWidth;
      const cssH = canvas.clientHeight;
      isWide = cssW >= COLLAPSED_THRESHOLD_PX;
      const w = Math.max(1, Math.floor(cssW * dpr));
      const h = Math.max(1, Math.floor(cssH * dpr));
      if (canvas.width !== w || canvas.height !== h) {
        canvas.width = w;
        canvas.height = h;
      }
      gl.viewport(0, 0, canvas.width, canvas.height);
    };

    const draw = () => {
      const p = propsRef.current;
      const t = THEMES[p.theme] ?? THEMES.blue;
      gl.useProgram(prog);
      gl.bindVertexArray(vao);
      gl.activeTexture(gl.TEXTURE0);
      gl.bindTexture(gl.TEXTURE_2D, tex);
      gl.uniform1i(U.texNoise, 0);
      gl.uniform1f(U.time, globalTime * 0.95);
      gl.uniform2f(U.viewport, canvas.width, canvas.height);
      gl.uniform3f(U.colMain, t.main[0], t.main[1], t.main[2]);
      gl.uniform3f(U.colLow, t.low[0], t.low[1], t.low[2]);
      gl.uniform3f(U.colMid, t.mid[0], t.mid[1], t.mid[2]);
      gl.uniform3f(U.colHigh, t.high[0], t.high[1], t.high[2]);
      gl.uniform1f(U.windSpeed, p.windSpeed);
      gl.uniform1f(U.warpPower, p.warpPower);
      gl.uniform1f(U.fbmStr, p.fbmStrength);
      gl.uniform1f(U.blurRad, p.blurRadius);
      gl.uniform1f(U.zoom, p.zoom);
      gl.uniform1f(U.grainStr, p.grainStrength);
      gl.uniform1f(U.grainSc, p.grainScale);
      gl.uniform1f(U.noiseSc, p.noiseScale);
      gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
      gl.bindVertexArray(null);
      drewOnce = true;
    };

    const ro = new ResizeObserver(() => {
      const wasWide = isWide;
      resize();
      // Re-kick the loop if we just expanded; force a redraw if the canvas
      // dimensions changed and we're collapsed (so the static frame
      // matches the new size).
      if (isWide && !wasWide && isVisible && !rafId) {
        lastTs = performance.now();
        rafId = requestAnimationFrame(frame);
      } else if (!isWide && drewOnce) {
        draw();
      }
    });
    ro.observe(canvas);
    resize();

    const io = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          const wasVisible = isVisible;
          isVisible = entry.isIntersecting;
          if (isVisible && !wasVisible) {
            lastTs = performance.now();
            if (!rafId && isWide && !reducedMotion) {
              rafId = requestAnimationFrame(frame);
            } else if (!drewOnce) {
              // Single static frame for collapsed-on-mount or reduced-motion
              // cards — without this they'd render as solid black.
              draw();
            }
          } else if (!isVisible && wasVisible && rafId) {
            cancelAnimationFrame(rafId);
            rafId = 0;
          }
        });
      },
      { threshold: 0 },
    );
    io.observe(canvas);

    const frame = (ts: number) => {
      if (!isVisible || !isWide || reducedMotion) {
        rafId = 0;
        if (!drewOnce) draw();
        return;
      }
      rafId = requestAnimationFrame(frame);
      // 30fps frame skip — still smooth, half the cost.
      if (ts - lastDrawTs < FRAME_BUDGET_MS) return;
      const delta = (ts - lastTs) / 1000;
      lastTs = ts;
      lastDrawTs = ts;
      const p = propsRef.current;
      globalTime += delta * p.speed;
      draw();
    };
    if (!reducedMotion) rafId = requestAnimationFrame(frame);
    else draw();

    return () => {
      if (rafId) cancelAnimationFrame(rafId);
      ro.disconnect();
      io.disconnect();
      gl.deleteBuffer(buf);
      gl.deleteVertexArray(vao);
      if (tex) gl.deleteTexture(tex);
      gl.deleteProgram(prog);
    };
  }, []);

  return (
    <canvas
      ref={canvasRef}
      aria-hidden
      className={className}
      style={{
        display: "block",
        width: "100%",
        height: "100%",
        pointerEvents: "none",
        ...style,
      }}
    />
  );
}
