"use client";

import { useRef, useMemo, useState } from "react";
import { Canvas, useFrame } from "@react-three/fiber";
import * as THREE from "three";

// SVG path data for the two arrows (scaled to normalized coordinates)
// Left arrow: points="80,180 240,256 80,332"
// Right arrow: points="432,180 272,256 432,332"
// Normalized to 0-1 range where 256 = 0.5

function ArrowLeft({ hovered }: { hovered: boolean }) {
  const meshRef = useRef<THREE.Mesh>(null);

  const shape = useMemo(() => {
    const s = new THREE.Shape();
    // Left arrow: pointing right (80,180 -> 240,256 -> 80,332)
    // Scaled to 0-1 range: 80/512=0.156, 180/512=0.352, 240/512=0.469, 256/512=0.5, 332/512=0.648
    s.moveTo(0.156, 0.352);
    s.lineTo(0.469, 0.5);
    s.lineTo(0.156, 0.648);
    s.lineTo(0.156, 0.352);
    return s;
  }, []);

  const geometry = useMemo(() => {
    return new THREE.ExtrudeGeometry(shape, {
      depth: 0.08,
      bevelEnabled: true,
      bevelThickness: 0.01,
      bevelSize: 0.01,
      bevelSegments: 4,
    });
  }, [shape]);

  // Ceramic material - matte, substantial
  const material = useMemo(() => {
    return new THREE.MeshStandardMaterial({
      color: new THREE.Color(0xf5f5f5),
      roughness: 0.4,
      metalness: 0.1,
    });
  }, []);

  useFrame((state) => {
    if (!meshRef.current) return;

    // Subtle floating animation
    const time = state.clock.elapsedTime;
    meshRef.current.position.y = Math.sin(time * 0.5) * 0.01;
    meshRef.current.rotation.z = Math.sin(time * 0.3) * 0.02;

    // Hover effect - move closer to center
    const targetX = hovered ? 0.02 : 0;
    meshRef.current.position.x = THREE.MathUtils.lerp(meshRef.current.position.x, targetX, 0.1);
  });

  return (
    <mesh
      ref={meshRef}
      geometry={geometry}
      material={material}
      position={[-0.05, 0, 0.04]}
      rotation={[0, 0.1, 0]}
    />
  );
}

function ArrowRight({ hovered }: { hovered: boolean }) {
  const meshRef = useRef<THREE.Mesh>(null);

  const shape = useMemo(() => {
    const s = new THREE.Shape();
    // Right arrow: pointing left (432,180 -> 272,256 -> 432,332)
    // 432/512=0.844, 272/512=0.531
    s.moveTo(0.844, 0.352);
    s.lineTo(0.531, 0.5);
    s.lineTo(0.844, 0.648);
    s.lineTo(0.844, 0.352);
    return s;
  }, []);

  const geometry = useMemo(() => {
    return new THREE.ExtrudeGeometry(shape, {
      depth: 0.06,
      bevelEnabled: true,
      bevelThickness: 0.008,
      bevelSize: 0.008,
      bevelSegments: 4,
    });
  }, [shape]);

  // Glass material - translucent with transmission
  const material = useMemo(() => {
    return new THREE.MeshPhysicalMaterial({
      color: new THREE.Color(0xffffff),
      metalness: 0,
      roughness: 0.05,
      transmission: 0.6,
      thickness: 0.1,
      opacity: 0.7,
      transparent: true,
      ior: 1.5,
      clearcoat: 1,
      clearcoatRoughness: 0.05,
    });
  }, []);

  useFrame((state) => {
    if (!meshRef.current) return;

    // Subtle floating animation with slight lag
    const time = state.clock.elapsedTime;
    meshRef.current.position.y = Math.sin(time * 0.5 - 0.5) * 0.012;
    meshRef.current.rotation.z = Math.sin(time * 0.3 - 0.3) * 0.015;

    // Hover effect - move closer to center
    const targetX = hovered ? -0.02 : 0;
    meshRef.current.position.x = THREE.MathUtils.lerp(meshRef.current.position.x, targetX, 0.08);
  });

  return (
    <mesh
      ref={meshRef}
      geometry={geometry}
      material={material}
      position={[0.05, 0, -0.02]}
      rotation={[0, -0.05, 0]}
    />
  );
}

function ImpactBurst({ hovered }: { hovered: boolean }) {
  const lightRef = useRef<THREE.PointLight>(null);
  const glowRef = useRef<THREE.Mesh>(null);
  const particlesRef = useRef<THREE.Points>(null);

  // Particle system for energy motes - deterministic pseudo-random
  const particleCount = 30;
  const positions = useMemo(() => {
    const arr = new Float32Array(particleCount * 3);
    for (let i = 0; i < particleCount; i++) {
      const angle = (i / particleCount) * Math.PI * 2;
      // Deterministic variation using sine of index
      const radiusVariation = Math.sin(i * 1.7) * 0.025 + 0.025; // 0 to 0.05
      const radius = 0.08 + radiusVariation;
      const zOffset = Math.sin(i * 2.3) * 0.025; // -0.025 to 0.025
      arr[i * 3] = Math.cos(angle) * radius;
      arr[i * 3 + 1] = Math.sin(angle) * radius;
      arr[i * 3 + 2] = zOffset;
    }
    return arr;
  }, []);

  useFrame((state) => {
    if (!lightRef.current || !glowRef.current || !particlesRef.current) return;

    const time = state.clock.elapsedTime;

    // Pulsing light intensity
    const baseIntensity = hovered ? 2.5 : 1.5;
    lightRef.current.intensity = baseIntensity + Math.sin(time * 3) * 0.5;

    // Glow scale pulsing
    const scale = 1 + Math.sin(time * 2) * 0.1;
    glowRef.current.scale.setScalar(scale);

    // Rotate particles
    particlesRef.current.rotation.z = time * 0.2;

    // Hover glow intensification
    const targetOpacity = hovered ? 0.9 : 0.6;
    const material = glowRef.current.material as THREE.MeshBasicMaterial;
    material.opacity = THREE.MathUtils.lerp(material.opacity, targetOpacity, 0.1);
  });

  return (
    <group>
      {/* Central point light - the energy source */}
      <pointLight
        ref={lightRef}
        position={[0, 0, 0.1]}
        color={new THREE.Color(0xff6b4a)}
        intensity={1.5}
        distance={0.8}
        decay={2}
      />

      {/* Glow sphere */}
      <mesh ref={glowRef} position={[0, 0, 0.05]}>
        <sphereGeometry args={[0.06, 32, 32]} />
        <meshBasicMaterial
          color={new THREE.Color(0xff6b4a)}
          transparent
          opacity={0.6}
        />
      </mesh>

      {/* Energy particles */}
      <points ref={particlesRef}>
        <bufferGeometry>
          <bufferAttribute
            attach="attributes-position"
            args={[positions, 3]}
          />
        </bufferGeometry>
        <pointsMaterial
          size={0.008}
          color={new THREE.Color(0xffaa88)}
          transparent
          opacity={0.8}
          sizeAttenuation
        />
      </points>
    </group>
  );
}

function Scene({ hovered }: { hovered: boolean }) {
  return (
    <>
      {/* Ambient light for base visibility */}
      <ambientLight intensity={0.4} />

      {/* Main directional light from above-left */}
      <directionalLight
        position={[-0.5, 0.8, 0.5]}
        intensity={1.2}
        color={new THREE.Color(0xffffff)}
      />

      {/* Accent rim light from below-right (that warm brand color) */}
      <spotLight
        position={[0.4, -0.6, 0.3]}
        target-position={[0, 0, 0]}
        intensity={hovered ? 3 : 1.5}
        color={new THREE.Color(0xd97757)}
        angle={0.5}
        penumbra={0.5}
        distance={2}
      />

      {/* Fill light from behind */}
      <pointLight
        position={[0, 0, -0.5]}
        intensity={0.3}
        color={new THREE.Color(0x445566)}
        distance={1.5}
      />

      {/* The arrows */}
      <ArrowLeft hovered={hovered} />
      <ArrowRight hovered={hovered} />

      {/* The energy burst at impact */}
      <ImpactBurst hovered={hovered} />
    </>
  );
}

export default function ClashIcon3D({
  size = 120,
  className = "",
}: {
  size?: number;
  className?: string;
}) {
  const [hovered, setHovered] = useState(false);

  return (
    <div
      className={className}
      style={{
        width: size,
        height: size,
        cursor: "pointer",
        borderRadius: "12px",
        overflow: "hidden",
      }}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      <Canvas
        camera={{
          position: [0, 0, 0.7],
          fov: 45,
          near: 0.1,
          far: 10,
        }}
        gl={{
          antialias: true,
          alpha: true,
          powerPreference: "high-performance",
        }}
        dpr={[1, 2]}
      >
        <Scene hovered={hovered} />
      </Canvas>
    </div>
  );
}
