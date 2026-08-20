'use client';

import { motion, type Variants } from 'motion/react';

import {
  getVariants,
  useAnimateIconContext,
  IconWrapper,
  type IconProps,
} from '@/components/animate-ui/icons/icon';

type GitForkProps = IconProps<keyof typeof animations>;

const animations = {
  default: {
    group: {
      initial: {
        scale: 1,
        transition: { type: 'spring', stiffness: 150, damping: 25 },
      },
      animate: {
        scale: 1.05,
        transition: { type: 'spring', stiffness: 150, damping: 25 },
      },
    },
    branchLeft: {
      initial: {
        x: 0,
        transition: { type: 'spring', stiffness: 150, damping: 25 },
      },
      animate: {
        x: -1.5,
        transition: { type: 'spring', stiffness: 150, damping: 25 },
      },
    },
    branchRight: {
      initial: {
        x: 0,
        transition: { type: 'spring', stiffness: 150, damping: 25 },
      },
      animate: {
        x: 1.5,
        transition: { type: 'spring', stiffness: 150, damping: 25 },
      },
    },
    commit: {},
    connector: {},
    stem: {},
  } satisfies Record<string, Variants>,
} as const;

function IconComponent({ size, ...props }: GitForkProps) {
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
      variants={variants.group}
      initial="initial"
      animate={controls}
      {...props}
    >
      <motion.circle
        cx="12"
        cy="18"
        r="3"
        variants={variants.commit}
        initial="initial"
        animate={controls}
      />
      <motion.circle
        cx="6"
        cy="6"
        r="3"
        variants={variants.branchLeft}
        initial="initial"
        animate={controls}
      />
      <motion.circle
        cx="18"
        cy="6"
        r="3"
        variants={variants.branchRight}
        initial="initial"
        animate={controls}
      />
      <motion.path
        d="M18 9v2c0 .6-.4 1-1 1H7c-.6 0-1-.4-1-1V9"
        variants={variants.connector}
        initial="initial"
        animate={controls}
      />
      <motion.path
        d="M12 12v3"
        variants={variants.stem}
        initial="initial"
        animate={controls}
      />
    </motion.svg>
  );
}

function GitFork(props: GitForkProps) {
  return <IconWrapper icon={IconComponent} {...props} />;
}

export {
  animations,
  GitFork,
  GitFork as GitForkIcon,
  type GitForkProps,
  type GitForkProps as GitForkIconProps,
};
