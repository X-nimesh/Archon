import { IsEmail, IsString, MaxLength, MinLength } from 'class-validator';

export class SignupDto {
  @IsString()
  @MinLength(2, { message: 'Name must be at least 2 characters long' })
  @MaxLength(80, { message: 'Name must be at most 80 characters long' })
  name!: string;

  @IsEmail({}, { message: 'Please enter a valid email address' })
  @MaxLength(254)
  email!: string;

  @IsString()
  @MinLength(8, { message: 'Password must be at least 8 characters long' })
  @MaxLength(128, { message: 'Password must be at most 128 characters long' })
  password!: string;
}