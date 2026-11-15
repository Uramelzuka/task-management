import { Platform } from 'react-native';

/**
 * Backend base URL.
 *
 * - Android emulator reaches the host machine at 10.0.2.2.
 * - iOS simulator shares the host network, so localhost works.
 * - Physical device: set API_URL_OVERRIDE to http://<your-LAN-IP>:8080.
 */
const API_URL_OVERRIDE: string | null = null;

const defaultBaseUrl = Platform.OS === 'android' ? 'http://10.0.2.2:8080' : 'http://localhost:8080';

export const API_BASE_URL = (API_URL_OVERRIDE ?? defaultBaseUrl).replace(/\/$/, '');
