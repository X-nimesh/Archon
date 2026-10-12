import * as z from 'zod';

export const LoginSchema = z.object({
  email: z.email({ error: 'Please enter a valid email address.' }).trim(),
  password: z.string().min(1, { error: 'Password is required.' }),
});

export const SignupSchema = z.object({
  name: z
    .string()
    .min(2, { error: 'Name must be at least 2 characters long.' })
    .max(80, { error: 'Name must be at most 80 characters long.' })
    .trim(),
  email: z.email({ error: 'Please enter a valid email address.' }).trim(),
  password: z
    .string()
    .min(8, { error: 'Be at least 8 characters long.' })
    .max(128, { error: 'Be at most 128 characters long.' }),
});

export type LoginInput = z.infer<typeof LoginSchema>;
export type SignupInput = z.infer<typeof SignupSchema>;

/** Per-field messages, keyed by input name. */
export type FieldErrors = Record<string, string[] | undefined>;