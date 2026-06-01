// React Native native-bridge (JS↔native boundary) fixture (#3580).
//
// Exercises the new-architecture + legacy bridge surfaces that emit
// SCOPE.External native-boundary entities + DEPENDS_ON edges:
//   - const { BiometricAuth } = NativeModules        → native_module "BiometricAuth"
//   - TurboModuleRegistry.getEnforcing('RNDeviceInfo') → native_module "RNDeviceInfo"
//   - requireNativeComponent('RCTMapView')           → native_component "RCTMapView"
//   - codegenNativeComponent<Props>('RCTWebView')    → native_component "RCTWebView"
//   - requireNativeModule('ExpoBattery')             → native_module "ExpoBattery"
import { NativeModules, TurboModuleRegistry, requireNativeComponent } from 'react-native';
import codegenNativeComponent from 'react-native/Libraries/Utilities/codegenNativeComponent';
import { requireNativeModule } from 'expo-modules-core';

const { BiometricAuth } = NativeModules;

const deviceInfo = TurboModuleRegistry.getEnforcing('RNDeviceInfo');

export const MapView = requireNativeComponent('RCTMapView');

export const WebView = codegenNativeComponent<{ src: string }>('RCTWebView');

const Battery = requireNativeModule('ExpoBattery');

export async function authenticate() {
  return BiometricAuth.prompt('Confirm identity');
}

export function model(): string {
  return deviceInfo.getModel();
}

export function level(): number {
  return Battery.getLevel();
}
