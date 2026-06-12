"use client";

import { useCallback, useEffect, useRef, useState } from "react";

const FRAGMENT_SHADER = `#version 300 es
precision highp float;
out vec4 O;
uniform float time;
uniform vec2 resolution;
uniform float intensity;
uniform float particleCount;
uniform vec2 tilt;
uniform vec3 hueA;
uniform vec3 hueB;

#define FC gl_FragCoord.xy
#define R  resolution
#define T  time

float rnd(float a) {
  vec2 p = fract(a * vec2(12.9898, 78.233));
  p += dot(p, p*345.);
  return fract(p.x * p.y);
}

vec3 hue(float a, vec3 tint) {
  return tint * (.6+.6*cos(6.3*(a)+vec3(0,83,21)));
}

vec3 streak(vec2 uv, vec3 tint, float phase, float speed) {
  vec3 col = vec3(0.);
  float t = T * speed + phase;
  for (float i=.0; i<particleCount; i++) {
    float a = rnd(i + phase * 13.);
    vec2 n = vec2(a, fract(a*34.56));
    vec2 p = sin(n*(t+7.) + t*.5);
    float d = dot(uv-p, uv-p);
    col += (intensity * .00125)/d * hue(dot(uv,uv) + i*.125 + t, tint);
  }
  return col;
}

void main(void) {
  vec2 fc = (FC - .5 * R) / min(R.x, R.y);
  fc -= tilt * 0.18;
  vec3 col = vec3(0.);
  float s = 2.4;
  float a = atan(fc.x, fc.y);
  float b = length(fc);

  // Streak A — leans one way, default cadence
  vec2 uvA = vec2(a * 5. / 6.28318 - 0.18, .05 / tan(b) + T);
  uvA = fract(uvA) - .5;
  col += streak(uvA * s, hueA, 0.0, 1.0);

  // Streak B — leans the other way, slightly faster so the two arcs cross
  vec2 uvB = vec2(a * 5. / 6.28318 + 0.18, .05 / tan(b) + T * 1.07);
  uvB = fract(uvB) - .5;
  col += streak(uvB * s, hueB, 1.7, 1.07);

  O = vec4(col, 1.);
}`;

const VERTEX_SHADER = `#version 300 es
precision highp float;
in vec2 position;
void main(){
  gl_Position = vec4(position, 0.0, 1.0);
}`;

type LightSpeedProps = {
  paused?: boolean;
  speed?: number;
  intensity?: number;
  particleCount?: number;
  hueA?: [number, number, number];
  hueB?: [number, number, number];
  quality?: "low" | "medium" | "high";
};

type Uniforms = {
  time: WebGLUniformLocation | null;
  resolution: WebGLUniformLocation | null;
  intensity: WebGLUniformLocation | null;
  particleCount: WebGLUniformLocation | null;
  tilt: WebGLUniformLocation | null;
  hueA: WebGLUniformLocation | null;
  hueB: WebGLUniformLocation | null;
};

const qualitySettings = {
  low: { dpr: 0.5, targetFps: 30 },
  medium: { dpr: 1, targetFps: 60 },
  high: { dpr: 1.5, targetFps: 60 },
};

const DEFAULT_HUE_A: [number, number, number] = [0.45, 0.7, 1.45];
const DEFAULT_HUE_B: [number, number, number] = [1.45, 0.45, 0.55];

const TILT_LERP = 0.08;
const GYRO_RANGE_DEG = 22;
const SPEED_LERP = 0.06;
const SPEED_BOOST_TARGET = 3.2;

type IOSOrientationCtor = {
  requestPermission?: () => Promise<"granted" | "denied">;
};

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}

function getIOSPermissionRequester(): (() => Promise<"granted" | "denied">) | null {
  if (typeof window === "undefined") return null;
  const ctor = window.DeviceOrientationEvent as
    | (typeof DeviceOrientationEvent & IOSOrientationCtor)
    | undefined;
  if (!ctor || typeof ctor.requestPermission !== "function") return null;
  return ctor.requestPermission.bind(ctor);
}

export function LightSpeed({
  paused = false,
  speed = 1,
  intensity = 1,
  particleCount = 20,
  hueA = DEFAULT_HUE_A,
  hueB = DEFAULT_HUE_B,
  quality = "medium",
}: LightSpeedProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const glRef = useRef<WebGL2RenderingContext | null>(null);
  const programRef = useRef<WebGLProgram | null>(null);
  const vboRef = useRef<WebGLBuffer | null>(null);
  const uniformsRef = useRef<Uniforms>({
    time: null,
    resolution: null,
    intensity: null,
    particleCount: null,
    tilt: null,
    hueA: null,
    hueB: null,
  });
  const rafRef = useRef(0);
  const lastFrameRef = useRef(0);
  const tiltTargetRef = useRef<[number, number]>([0, 0]);
  const tiltCurrentRef = useRef<[number, number]>([0, 0]);
  const attachOrientationRef = useRef<(() => void) | null>(null);
  const warpHoveringRef = useRef(false);
  const speedCurrentRef = useRef(1);
  const [webglOk, setWebglOk] = useState(true);
  const [shaderReady, setShaderReady] = useState(false);
  const [tiltAvailable, setTiltAvailable] = useState(false);
  const [tiltGranted, setTiltGranted] = useState(false);
  const currentQuality = qualitySettings[quality] ?? qualitySettings.medium;
  const [hueAR, hueAG, hueAB] = hueA;
  const [hueBR, hueBG, hueBB] = hueB;

  // Pointer + gyro listeners — independent of WebGL availability so that the
  // chip and motion intent still work if the shader pass falls back.
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const onPointerMove = (event: PointerEvent) => {
      const rect = container.getBoundingClientRect();
      if (rect.width === 0 || rect.height === 0) return;
      const x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
      const y = ((event.clientY - rect.top) / rect.height) * 2 - 1;
      tiltTargetRef.current = [clamp(x, -1, 1), clamp(y, -1, 1)];
    };

    const onPointerLeave = () => {
      tiltTargetRef.current = [0, 0];
    };

    container.addEventListener("pointermove", onPointerMove, { passive: true });
    container.addEventListener("pointerleave", onPointerLeave, {
      passive: true,
    });

    const onOrientation = (event: DeviceOrientationEvent) => {
      if (event.gamma == null || event.beta == null) return;
      const x = clamp(event.gamma / GYRO_RANGE_DEG, -1, 1);
      const y = clamp(event.beta / GYRO_RANGE_DEG, -1, 1);
      tiltTargetRef.current = [x, y];
    };

    let orientationListenerAttached = false;
    const attachOrientation = () => {
      if (orientationListenerAttached) return;
      window.addEventListener("deviceorientation", onOrientation);
      orientationListenerAttached = true;
    };
    attachOrientationRef.current = attachOrientation;

    if (typeof window !== "undefined" && "DeviceOrientationEvent" in window) {
      setTiltAvailable(true);
      const requester = getIOSPermissionRequester();
      if (!requester) {
        // Android / desktop: no permission gate.
        attachOrientation();
        setTiltGranted(true);
      }
    }

    return () => {
      container.removeEventListener("pointermove", onPointerMove);
      container.removeEventListener("pointerleave", onPointerLeave);
      if (orientationListenerAttached) {
        window.removeEventListener("deviceorientation", onOrientation);
      }
      attachOrientationRef.current = null;
    };
  }, []);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const gl = canvas.getContext("webgl2", {
      alpha: false,
      antialias: false,
      depth: false,
      stencil: false,
      powerPreference: "high-performance",
    });

    if (!gl) {
      setWebglOk(false);
      return;
    }

    const compileShader = (type: number, source: string) => {
      const shader = gl.createShader(type);
      if (!shader) throw new Error("Unable to create shader");

      gl.shaderSource(shader, source);
      gl.compileShader(shader);
      if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
        const message = gl.getShaderInfoLog(shader) ?? "Shader compile error";
        gl.deleteShader(shader);
        throw new Error(message);
      }

      return shader;
    };

    const linkProgram = (vertex: WebGLShader, fragment: WebGLShader) => {
      const program = gl.createProgram();
      if (!program) throw new Error("Unable to create WebGL program");

      gl.attachShader(program, vertex);
      gl.attachShader(program, fragment);
      gl.linkProgram(program);
      if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
        const message = gl.getProgramInfoLog(program) ?? "Program link error";
        gl.deleteProgram(program);
        throw new Error(message);
      }

      return program;
    };

    try {
      const vertex = compileShader(gl.VERTEX_SHADER, VERTEX_SHADER);
      const fragment = compileShader(gl.FRAGMENT_SHADER, FRAGMENT_SHADER);
      const program = linkProgram(vertex, fragment);
      programRef.current = program;
      glRef.current = gl;
      gl.useProgram(program);

      const vbo = gl.createBuffer();
      vboRef.current = vbo;
      gl.bindBuffer(gl.ARRAY_BUFFER, vbo);
      gl.bufferData(
        gl.ARRAY_BUFFER,
        new Float32Array([-1, 1, -1, -1, 1, 1, 1, -1]),
        gl.STATIC_DRAW,
      );

      const position = gl.getAttribLocation(program, "position");
      gl.enableVertexAttribArray(position);
      gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0);

      uniformsRef.current = {
        time: gl.getUniformLocation(program, "time"),
        resolution: gl.getUniformLocation(program, "resolution"),
        intensity: gl.getUniformLocation(program, "intensity"),
        particleCount: gl.getUniformLocation(program, "particleCount"),
        tilt: gl.getUniformLocation(program, "tilt"),
        hueA: gl.getUniformLocation(program, "hueA"),
        hueB: gl.getUniformLocation(program, "hueB"),
      };
      setWebglOk(true);
    } catch {
      setWebglOk(false);
      return;
    }

    const resize = () => {
      const dpr = Math.max(
        1,
        Math.min(window.devicePixelRatio || 1, currentQuality.dpr),
      );
      const cssW = canvas.clientWidth || canvas.parentElement?.clientWidth || 1;
      const cssH =
        canvas.clientHeight || canvas.parentElement?.clientHeight || 1;
      canvas.width = Math.floor(cssW * dpr);
      canvas.height = Math.floor(cssH * dpr);
      gl.viewport(0, 0, canvas.width, canvas.height);
      gl.uniform2f(uniformsRef.current.resolution, canvas.width, canvas.height);
    };

    const observer =
      typeof ResizeObserver === "undefined"
        ? null
        : new ResizeObserver(resize);
    observer?.observe(canvas);
    window.addEventListener("resize", resize);
    resize();

    const start = performance.now();
    let firstDrawSeen = false;
    let shaderTime = 0;
    let lastTimestamp = 0;
    const loop = (timestamp: number) => {
      rafRef.current = requestAnimationFrame(loop);
      if (paused) return;

      const delta = timestamp - lastFrameRef.current;
      const targetFrameTime = 1000 / currentQuality.targetFps;
      if (delta < targetFrameTime) return;

      lastFrameRef.current = timestamp - (delta % targetFrameTime);
      const program = programRef.current;
      if (!program) return;

      // Speed boost: ease toward SPEED_BOOST_TARGET while pointer hovers,
      // back to 1 once it leaves.
      const speedTarget =
        (speed || 1) * (warpHoveringRef.current ? SPEED_BOOST_TARGET : 1);
      speedCurrentRef.current +=
        (speedTarget - speedCurrentRef.current) * SPEED_LERP;

      // Integrate time using current speed so the boost shows as a true warp.
      const dtSeconds = lastTimestamp === 0 ? 0 : (timestamp - lastTimestamp) / 1000;
      lastTimestamp = timestamp;
      shaderTime += dtSeconds * speedCurrentRef.current;
      // Anchor first frame near the original phase so the visual matches what
      // existed before the integrated-time refactor.
      if (!firstDrawSeen) shaderTime = (timestamp - start) * 0.001 * (speed || 1);

      const [tx, ty] = tiltTargetRef.current;
      const [cx, cy] = tiltCurrentRef.current;
      tiltCurrentRef.current = [
        cx + (tx - cx) * TILT_LERP,
        cy + (ty - cy) * TILT_LERP,
      ];

      gl.useProgram(program);
      gl.uniform1f(uniformsRef.current.time, shaderTime);
      gl.uniform1f(uniformsRef.current.intensity, intensity);
      gl.uniform1f(uniformsRef.current.particleCount, particleCount);
      gl.uniform2f(
        uniformsRef.current.tilt,
        tiltCurrentRef.current[0],
        tiltCurrentRef.current[1],
      );
      gl.uniform3f(uniformsRef.current.hueA, hueAR, hueAG, hueAB);
      gl.uniform3f(uniformsRef.current.hueB, hueBR, hueBG, hueBB);
      gl.clearColor(0, 0, 0, 1);
      gl.clear(gl.COLOR_BUFFER_BIT);
      gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
      if (!firstDrawSeen) {
        firstDrawSeen = true;
        setShaderReady(true);
      }
    };

    rafRef.current = requestAnimationFrame(loop);

    return () => {
      cancelAnimationFrame(rafRef.current);
      observer?.disconnect();
      window.removeEventListener("resize", resize);

      const program = programRef.current;
      if (program) {
        const shaders = gl.getAttachedShaders(program) ?? [];
        shaders.forEach((shader) => gl.deleteShader(shader));
        gl.deleteProgram(program);
      }
      if (vboRef.current) gl.deleteBuffer(vboRef.current);
    };
  }, [
    currentQuality.dpr,
    currentQuality.targetFps,
    hueAR,
    hueAG,
    hueAB,
    hueBR,
    hueBG,
    hueBB,
    intensity,
    particleCount,
    paused,
    speed,
  ]);

  const onWarpHoverStart = useCallback(() => {
    warpHoveringRef.current = true;
  }, []);
  const onWarpHoverEnd = useCallback(() => {
    warpHoveringRef.current = false;
  }, []);

  const onTiltChipClick = useCallback(async (event: React.MouseEvent) => {
    event.stopPropagation();
    const requester = getIOSPermissionRequester();
    if (!requester) return;
    try {
      const result = await requester();
      if (result === "granted") {
        attachOrientationRef.current?.();
        setTiltGranted(true);
      }
    } catch {
      // Permission failed; chip stays visible so user can retry.
    }
  }, []);

  const showTiltChip =
    tiltAvailable && !tiltGranted && getIOSPermissionRequester() !== null;

  return (
    <div
      ref={containerRef}
      aria-label="Lightspeed visual"
      onPointerEnter={onWarpHoverStart}
      onPointerLeave={onWarpHoverEnd}
      className="relative h-full min-h-[260px] w-full min-w-[100px] overflow-hidden bg-black"
      data-testid="lightspeed-visual"
    >
      {!webglOk && (
        <div className="absolute inset-0 grid place-items-center px-6 text-center text-neutral-200">
          <div className="max-w-md">
            <h2 className="text-xl font-semibold">WebGL not supported</h2>
            <p className="mt-2 text-sm text-white/70">
              Your browser or device does not support WebGL 2.0.
            </p>
          </div>
        </div>
      )}
      <canvas
        ref={canvasRef}
        data-shader-ready={shaderReady ? "true" : "false"}
        className={`absolute inset-0 block h-full w-full transition-opacity duration-700 ease-out ${
          shaderReady ? "opacity-100" : "opacity-0"
        }`}
        data-testid="lightspeed-canvas"
      />
      {showTiltChip && (
        <button
          type="button"
          onClick={onTiltChipClick}
          data-testid="lightspeed-tilt-chip"
          className="absolute left-4 top-4 z-10 rounded-full border border-white/15 bg-white/[0.06] px-3 py-1.5 font-mono text-2xs uppercase tracking-[0.2em] text-white/70 backdrop-blur transition-colors hover:bg-white/[0.1] hover:text-white"
        >
          Tap to tilt
        </button>
      )}
    </div>
  );
}
