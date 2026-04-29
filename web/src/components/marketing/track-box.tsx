"use client";

/*
 * TrackBox — 3D version of TransparentFrame from landing.tsx.
 *
 * A wireframe cuboid (the sandbox/run boundary) with five parallel lanes
 * piercing through it along the X axis at different Y heights, and a
 * white streak per lane traveling left-to-right. Tilts toward the cursor
 * on hover (±12° on rotateX/rotateY) and eases back to flat on leave.
 *
 * The flat 2D `TransparentFrame` in landing.tsx is preserved verbatim;
 * to revert, swap `<TrackBox />` back to `<TransparentFrame />` at the
 * single call site.
 */

import { Canvas, useFrame } from "@react-three/fiber";
import { useMemo, useRef } from "react";
import * as THREE from "three";
import type { Group, Mesh } from "three";

const BOX_W = 3.2;
const BOX_H = 1.7;
const BOX_D = 1.4;
const LANE_COUNT = 5;
// Lanes thread through the box on the X axis. Streaks travel a slightly
// longer X span so they enter from off-screen and exit off-screen.
const LANE_X_SPAN = BOX_W * 1.45;

export function TrackBox({ className }: { className?: string }) {
  const containerRef = useRef<HTMLDivElement>(null);
  // Target rotation tracks the cursor; the actual rotation lerps toward
  // it inside the rAF loop. Stored in a ref so updates don't re-render.
  const targetRot = useRef({ x: 0, y: 0 });

  const onPointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    const el = containerRef.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    const nx = (e.clientX - rect.left) / rect.width - 0.5; // -0.5..0.5
    const ny = (e.clientY - rect.top) / rect.height - 0.5;
    // 0.21 rad ≈ ±12°. Y-cursor inverts to rotateX so up = tip-back.
    targetRot.current.y = nx * 0.42;
    targetRot.current.x = -ny * 0.42;
  };
  const onPointerLeave = () => {
    targetRot.current.x = 0;
    targetRot.current.y = 0;
  };

  return (
    <div
      ref={containerRef}
      className={className}
      onPointerMove={onPointerMove}
      onPointerLeave={onPointerLeave}
      style={{ width: "100%", aspectRatio: "4 / 3", maxWidth: 480 }}
    >
      <Canvas
        camera={{ position: [0, 0, 4.5], fov: 38 }}
        dpr={[1, 1.5]}
        gl={{ antialias: true, alpha: true }}
      >
        <Scene targetRot={targetRot} />
      </Canvas>
    </div>
  );
}

function Scene({
  targetRot,
}: {
  targetRot: React.RefObject<{ x: number; y: number }>;
}) {
  const groupRef = useRef<Group>(null);

  useFrame(() => {
    const g = groupRef.current;
    if (!g) return;
    // Critically-damped easing so it always settles, never wobbles.
    g.rotation.x += (targetRot.current!.x - g.rotation.x) * 0.12;
    g.rotation.y += (targetRot.current!.y - g.rotation.y) * 0.12;
  });

  // Lane Y positions, evenly spaced inside the box (with a small inset
  // from the top/bottom faces).
  const laneYs = useMemo(() => {
    const inset = 0.18;
    const top = BOX_H / 2 - inset;
    const bottom = -BOX_H / 2 + inset;
    const span = top - bottom;
    return Array.from(
      { length: LANE_COUNT },
      (_, i) => bottom + (span * i) / (LANE_COUNT - 1),
    );
  }, []);

  return (
    <group ref={groupRef}>
      {/* Outer wireframe boundary — opacity matches the 2D TransparentFrame
          stroke (rgba(255,255,255,0.35)) so the 3D version reads with the
          same restrained brand weight. */}
      <BoxFrame
        width={BOX_W}
        height={BOX_H}
        depth={BOX_D}
        color="#ffffff"
        opacity={0.35}
      />
      {/* Inner dashed boundary — opacity 0.08, same as the 2D inner rect. */}
      <DashedBoxFrame
        width={BOX_W * 0.92}
        height={BOX_H * 0.86}
        depth={BOX_D * 0.86}
        color="#ffffff"
        opacity={0.08}
      />
      {laneYs.map((y, i) => (
        <Lane key={i} y={y} delay={(i / LANE_COUNT) * 1.6} />
      ))}
    </group>
  );
}

function BoxFrame({
  width,
  height,
  depth,
  color,
  opacity,
}: {
  width: number;
  height: number;
  depth: number;
  color: string;
  opacity: number;
}) {
  const geometry = useMemo(() => {
    return new THREE.EdgesGeometry(
      new THREE.BoxGeometry(width, height, depth),
    );
  }, [width, height, depth]);
  return (
    <lineSegments geometry={geometry}>
      <lineBasicMaterial color={color} transparent opacity={opacity} />
    </lineSegments>
  );
}

function DashedBoxFrame({
  width,
  height,
  depth,
  color,
  opacity,
}: {
  width: number;
  height: number;
  depth: number;
  color: string;
  opacity: number;
}) {
  const geometry = useMemo(() => {
    return new THREE.EdgesGeometry(
      new THREE.BoxGeometry(width, height, depth),
    );
  }, [width, height, depth]);
  // computeLineDistances is required for LineDashedMaterial.
  geometry.computeBoundingBox();
  return (
    <lineSegments
      geometry={geometry}
      onUpdate={(self) => {
        // R3F doesn't auto-call computeLineDistances; do it after mount.
        (self as unknown as THREE.LineSegments).computeLineDistances();
      }}
    >
      <lineDashedMaterial
        color={color}
        transparent
        opacity={opacity}
        dashSize={0.08}
        gapSize={0.08}
      />
    </lineSegments>
  );
}

function Lane({ y, delay }: { y: number; delay: number }) {
  const streakRef = useRef<Mesh>(null);

  // Static line geometry: a single segment along X at this Y, z=0.
  const lineGeom = useMemo(() => {
    const g = new THREE.BufferGeometry();
    const half = LANE_X_SPAN / 2;
    g.setAttribute(
      "position",
      new THREE.Float32BufferAttribute([-half, y, 0, half, y, 0], 3),
    );
    return g;
  }, [y]);

  useFrame(({ clock }) => {
    const m = streakRef.current;
    if (!m) return;
    // 4-second cycle; offset per lane via `delay`.
    const period = 4;
    const t = ((clock.elapsedTime + delay) % period) / period;
    const half = LANE_X_SPAN / 2;
    m.position.x = -half + t * LANE_X_SPAN;
    m.position.y = y;
    m.position.z = 0;
  });

  return (
    <>
      <line
        // @ts-expect-error — R3F overloads <line> as a three.js Line
        // primitive. TS picks up the SVG <line> typing instead.
        geometry={lineGeom}
      >
        <lineBasicMaterial color="#ffffff" transparent opacity={0.05} />
      </line>
      <mesh ref={streakRef}>
        <boxGeometry args={[0.22, 0.025, 0.025]} />
        <meshBasicMaterial color="#ffffff" />
      </mesh>
    </>
  );
}
