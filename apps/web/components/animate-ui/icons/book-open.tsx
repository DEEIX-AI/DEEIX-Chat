'use client';

import { motion, type Variants } from 'motion/react';

import {
  getVariants,
  type IconProps,
  IconWrapper,
  useAnimateIconContext,
} from '@/components/animate-ui/icons/icon';

type BookOpenProps = IconProps<keyof typeof animations>;

const animations = {
  default: {
    leftPage: {
      initial: {
        opacity: 1,
        d: 'M12 7C11 4.8 9.7 3 7 3H2V18H9C10.6 18 11.5 19 12 21Z',
      },
      animate: {
        opacity: [1, 0.65, 0, 0.65, 1],
        d: [
          'M12 7C11 4.8 9.7 3 7 3H2V18H9C10.6 18 11.5 19 12 21Z',
          'M12 6C11.5 4.9 10.7 4 9.5 4H6V19H10C11 19 11.6 20 12 21Z',
          'M12 5C11.9 4.7 11.8 4 11.7 4H11.5V20H11.7C11.8 20 11.9 20.5 12 21Z',
          'M12 6C11.5 4.9 10.7 4 9.5 4H6V19H10C11 19 11.6 20 12 21Z',
          'M12 7C11 4.8 9.7 3 7 3H2V18H9C10.6 18 11.5 19 12 21Z',
        ],
        transition: {
          duration: 0.5,
          ease: 'easeInOut',
          times: [0, 0.22, 0.5, 0.78, 1],
        },
      },
    },
    rightPage: {
      initial: {
        opacity: 1,
        d: 'M12 7C13 4.8 14.3 3 17 3H22V18H15C13.4 18 12.5 19 12 21Z',
      },
      animate: {
        opacity: [1, 0.65, 0, 0.65, 1],
        d: [
          'M12 7C13 4.8 14.3 3 17 3H22V18H15C13.4 18 12.5 19 12 21Z',
          'M12 6C12.5 4.9 13.3 4 14.5 4H18V19H14C13 19 12.4 20 12 21Z',
          'M12 5C12.1 4.7 12.2 4 12.3 4H12.5V20H12.3C12.2 20 12.1 20.5 12 21Z',
          'M12 6C12.5 4.9 13.3 4 14.5 4H18V19H14C13 19 12.4 20 12 21Z',
          'M12 7C13 4.8 14.3 3 17 3H22V18H15C13.4 18 12.5 19 12 21Z',
        ],
        transition: {
          duration: 0.5,
          ease: 'easeInOut',
          times: [0, 0.22, 0.5, 0.78, 1],
        },
      },
    },
    closedCover: {
      initial: { opacity: 0, scaleX: 0.78 },
      animate: {
        opacity: [0, 0.2, 1, 0.2, 0],
        scaleX: [0.78, 0.9, 1, 0.9, 0.78],
        transition: {
          duration: 0.5,
          ease: 'easeInOut',
          times: [0, 0.22, 0.5, 0.78, 1],
        },
      },
    },
  } satisfies Record<string, Variants>,
} as const;

function IconComponent({ size, ...props }: BookOpenProps) {
  const { controls } = useAnimateIconContext();
  const variants = getVariants(animations);

  return (
    <motion.svg
      xmlns="http://www.w3.org/2000/svg"
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      strokeLinecap="round"
      strokeLinejoin="round"
      {...props}
    >
      <motion.path
        d="M12 7C11 4.8 9.7 3 7 3H2V18H9C10.6 18 11.5 19 12 21Z"
        opacity={1}
        variants={variants.leftPage}
        initial="initial"
        animate={controls}
      />
      <motion.path
        d="M12 7C13 4.8 14.3 3 17 3H22V18H15C13.4 18 12.5 19 12 21Z"
        opacity={1}
        variants={variants.rightPage}
        initial="initial"
        animate={controls}
      />
      <motion.path
        d="M8 3h9v18H8a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1Z"
        fill="currentColor"
        fillOpacity={0.08}
        opacity={0}
        variants={variants.closedCover}
        initial="initial"
        animate={controls}
        style={{ transformOrigin: '12px 12px' }}
      />
    </motion.svg>
  );
}

function BookOpen(props: BookOpenProps) {
  return <IconWrapper icon={IconComponent} {...props} />;
}

export {
  animations,
  BookOpen,
  BookOpen as BookOpenIcon,
  type BookOpenProps,
  type BookOpenProps as BookOpenIconProps,
};
