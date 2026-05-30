print("case:vec_color_geometry_hsl_more")

func near(a, b) {
  return math.abs(a - b) <= 0.000001
}

func nearVec2(v, x, y) {
  return near(v.x, x) && near(v.y, y)
}

func nearColor(c, r, g, b, a) {
  return near(c.r, r) && near(c.g, g) && near(c.b, b) && near(c.a, a)
}

assert(near(vec.angle2(vec.vec2(0, 1)), math.pi / 2))
assert(near(vec.angle2(vec.vec2(-1, 0)), math.pi))

r90 := vec.rotate2(vec.vec2(2, 0), math.pi / 2)
assert(nearVec2(r90, 0, 2))
r180 := vec.rotate2(vec.vec2(3, -2), math.pi)
assert(nearVec2(r180, -3, 2))

a := vec.vec2(-1, -2)
b := vec.vec2(2, 2)
assert(near(vec.dist2(a, b), 5))
assert(near(vec.distSq2(a, b), 25))

floorBounce := vec.reflect2(vec.vec2(3, -4), vec.vec2(0, 1))
assert(nearVec2(floorBounce, 3, 4))
diagNormal := vec.normalize2(vec.vec2(1, 1))
diagBounce := vec.reflect2(vec.vec2(1, 0), diagNormal)
assert(nearVec2(diagBounce, 0, -1))

clamped := vec.clamp2(vec.vec2(-3, 9), vec.vec2(-2, 1), vec.vec2(2, 4))
assert(nearVec2(clamped, -2, 4))
assert(near(vec.dist3(vec.vec3(1, 2, 3), vec.vec3(5, 5, 3)), 5))

hsl := color.fromHSL(210, 0.5, 0.4)
assert(nearColor(hsl, 0.2, 0.4, 0.6, 1))
h, s, l := color.toHSL(hsl)
assert(near(h, 210))
assert(near(s, 0.5))
assert(near(l, 0.4))

hsv := color.fromHSV(330, 0.75, 0.8)
assert(nearColor(hsv, 0.8, 0.2, 0.5, 1))
hh, ss, vv := color.toHSV(hsv)
assert(near(hh, 330))
assert(near(ss, 0.75))
assert(near(vv, 0.8))

base := color.new(0.2, 0.4, 0.6, 0.7)
light := color.lighten(base, 0.25)
assert(nearColor(light, 0.4, 0.55, 0.7, 0.7))
dark := color.darken(base, 0.25)
assert(nearColor(dark, 0.15, 0.3, 0.45, 0.7))
gray := color.grayscale(base)
assert(nearColor(gray, 0.37192, 0.37192, 0.37192, 0.7))

mixed := color.mix(color.new(1, 0.2, 0, 0.4), color.new(0, 0.6, 1, 1), 0.25)
assert(nearColor(mixed, 0.75, 0.3, 0.25, 0.55))
alpha := color.withAlpha(mixed, 0.125)
assert(nearColor(alpha, 0.75, 0.3, 0.25, 0.125))

print("ok")
