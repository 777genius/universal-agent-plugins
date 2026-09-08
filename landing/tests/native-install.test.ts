import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { describe, it } from 'node:test';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import Bowser from 'bowser';
import { applyCliInvocation, getCliInvocation } from '../utils/cliInvocation.ts';
import { normalizeInstallPlatform, recommendedInstallChannelId } from '../utils/installPlatform.ts';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const locales = ['en', 'es', 'fr', 'ru', 'zh'];

describe('native-first installation', () => {
  it('keeps the native Go CLI as the default command invocation', () => {
    assert.equal(getCliInvocation(), 'agentplugins');
    assert.equal(
      applyCliInvocation('universal-agent-plugins add context7'),
      'agentplugins add context7',
    );
    assert.equal(
      applyCliInvocation('universal-agent-plugins add context7', 'npx universal-agent-plugins'),
      'npx universal-agent-plugins add context7',
    );
  });

  it('publishes the same native and zero-install channels in every locale', () => {
    for (const locale of locales) {
      const content = JSON.parse(
        readFileSync(resolve(root, 'content', `${locale}.json`), 'utf8'),
      ) as {
        installChannels: Array<{
          id: string;
          command?: string;
          invocation?: string;
          recommended?: boolean;
        }>;
      };
      const selected = content.installChannels.filter((channel) =>
        ['brew', 'script', 'powershell', 'npm'].includes(channel.id),
      );
      assert.deepEqual(
        selected.map((channel) => channel.id),
        ['brew', 'npm', 'script', 'powershell'],
        locale,
      );
      assert.equal(selected.filter((channel) => channel.recommended).length, 1, locale);
      assert.equal(selected[0]?.recommended, true, locale);
      assert.equal(selected[0]?.command, 'brew install 777genius/agentplugins/agentplugins');
      assert.equal(selected[0]?.invocation, 'agentplugins');
      assert.equal(selected[1]?.invocation, 'npx universal-agent-plugins');
      assert.equal(selected[2]?.invocation, '$HOME/.local/bin/agentplugins');
      assert.equal(selected[3]?.invocation, '& "$HOME\\.local\\bin\\agentplugins.exe"');
      assert.ok(
        selected.every((channel) => !channel.command?.includes('plugin-kit-ai')),
        locale,
      );
    }
  });

  it('maps current browser OS names to the safest default channel', () => {
    const userAgents = {
      macos:
        'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/140.0.0.0 Safari/537.36',
      windows:
        'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/140.0.0.0 Safari/537.36',
      linux: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/140.0.0.0 Safari/537.36',
      mobile:
        'Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 Version/18.6 Mobile/15E148 Safari/604.1',
      android:
        'Mozilla/5.0 (Linux; Android 16; Pixel 10 Pro) AppleWebKit/537.36 Chrome/140.0.0.0 Mobile Safari/537.36',
    };

    for (const [expectedPlatform, userAgent] of Object.entries(userAgents)) {
      const platform = normalizeInstallPlatform(Bowser.getParser(userAgent).getOSName(true));
      assert.equal(platform, expectedPlatform === 'android' ? 'mobile' : expectedPlatform);
    }

    assert.equal(normalizeInstallPlatform('Android Linux'), 'mobile');

    assert.equal(recommendedInstallChannelId('macos'), 'brew');
    assert.equal(recommendedInstallChannelId('linux'), 'script');
    assert.equal(recommendedInstallChannelId('windows'), 'powershell');
    assert.equal(recommendedInstallChannelId('mobile'), null);
    assert.equal(recommendedInstallChannelId('other'), 'npm');
  });
});
