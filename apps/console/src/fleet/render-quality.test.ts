import { expect, test } from "vitest";
import { createRenderQuality } from "./render-quality";

test("mobile starts within one device pixel and desktop caps at one and a half", () => {
  expect(createRenderQuality(3, 390).pixelRatio).toBe(1);
  expect(createRenderQuality(3, 1440).pixelRatio).toBe(1.5);
  expect(createRenderQuality(0.8, 1440).pixelRatio).toBe(0.8);
});

test("twelve measured slow frames reduce resolution in bounded steps", () => {
  const quality = createRenderQuality(2, 1440);
  for (let index = 0; index < 12; index++) quality.frame(index * 40);
  expect(quality.pixelRatio).toBe(1.5);
  quality.frame(480);
  expect(quality.pixelRatio).toBe(1);
  for (let index = 13; index <= 60; index++) quality.frame(index * 40);
  expect(quality.pixelRatio).toBe(0.75);
});

test("fast frames retain resolution without oscillating after a downgrade", () => {
  const quality = createRenderQuality(2, 1440);
  for (let index = 0; index <= 60; index++) quality.frame(index * 16);
  expect(quality.pixelRatio).toBe(1.5);
  quality.frame();
  for (let index = 0; index <= 12; index++) quality.frame(index * 40);
  expect(quality.pixelRatio).toBe(1);
  quality.frame();
  for (let index = 0; index <= 60; index++) quality.frame(index * 16);
  expect(quality.pixelRatio).toBe(1);
});

test("idle time and interrupted highlights never count as slow active frames", () => {
  const quality = createRenderQuality(2, 1440);
  for (let index = 0; index < 11; index++) quality.frame(index * 100);
  quality.frame();
  for (let index = 0; index < 60; index++) quality.frame(100000 + index * 16);
  expect(quality.pixelRatio).toBe(1.5);
});

test("mobile accepts thirty frames per second and respects low device density", () => {
  const mobile = createRenderQuality(3, 390);
  for (let index = 0; index <= 24; index++) mobile.frame(index * 32);
  expect(mobile.pixelRatio).toBe(1);
  for (let index = 25; index <= 48; index++) mobile.frame(index * 50);
  expect(mobile.pixelRatio).toBe(0.75);
  const lowDensity = createRenderQuality(0.5, 1440);
  for (let index = 0; index <= 24; index++) lowDensity.frame(index * 50);
  expect(lowDensity.pixelRatio).toBe(0.5);
});
