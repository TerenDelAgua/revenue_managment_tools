import { writable } from 'svelte/store';

export type ToastType = 'success' | 'error' | 'info';

export interface Toast {
    id: string;
    message: string;
    type: ToastType;
    duration?: number;
}

export const toasts = writable<Toast[]>([]);

export function addToast(message: string, type: ToastType = 'info', duration = 4000) {
    const id = Math.random().toString(36).substring(2, 9);

    toasts.update((currentToasts) => {
        return [...currentToasts, { id, message, type, duration }];
    });

    // Auto-remove after duration
    setTimeout(() => {
        removeToast(id);
    }, duration);
}

export function removeToast(id: string) {
    toasts.update((currentToasts) => {
        return currentToasts.filter((t) => t.id !== id);
    });
}