import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * Merge Tailwind CSS class names with conditional logic support.
 * Wraps clsx + twMerge so conflicting Tailwind classes are resolved correctly.
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
