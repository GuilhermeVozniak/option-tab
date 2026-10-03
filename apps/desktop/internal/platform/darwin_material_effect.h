#ifndef OT_MATERIAL_EFFECT_H
#define OT_MATERIAL_EFFECT_H
#import <Cocoa/Cocoa.h>
// Shared visual recipe only. Host input/Space/lifecycle policy remains
// separate.
static inline void OTConfigureMaterialEffect(NSVisualEffectView *effect) {
  effect.material = NSVisualEffectMaterialHUDWindow;
  effect.blendingMode = NSVisualEffectBlendingModeBehindWindow;
  effect.state = NSVisualEffectStateActive;
}
#endif
